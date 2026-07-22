// Package blockvolume holds what every Oracle Cloud (OCI) Block Volumes action
// shares: the API-signing-key credential, BOTH service-client factories this node
// needs, the two per-resource client preambles, the resource summarisers, and the
// input/result helpers. Like the sibling OCI packages it has no Execute function, so
// the manifest generator skips it — but its category.go supplies the "Block Volumes"
// sub-category.
//
// This is the first OCI node that spans TWO service clients: volume / backup /
// group / policy CRUD lives on core.BlockstorageClient, while attach / detach and
// the attachment reads live on core.ComputeClient. The Compute node's common.go
// already exposes both factories on one Auth; this package lifts both and adds a
// per-resource preamble for each (VolumeResourceClient → Blockstorage,
// ComputeResourceClient → Compute). The auth block is the OCI signing-key model,
// identical to the other four OCI nodes. As with the siblings, the manifest
// generator only resolves INLINE Inputs literals, so the credential + compartment
// input declarations must still be copy-pasted into each action's Inputs.
package blockvolume

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"

	coreflow "flomation.app/automate/executor"
)

const (
	InputTenancyOCID     = "tenancy_ocid"
	InputUserOCID        = "user_ocid"
	InputRegion          = "region"
	InputFingerprint     = "fingerprint"
	InputPrivateKey      = "private_key"
	InputPassphrase      = "private_key_passphrase"
	InputCompartmentOCID = "compartment_ocid"
)

// ListMaxPages bounds every list action's pagination walk so one node run can't
// turn into an unbounded sequence of API calls; a walk that hits the cap sets
// truncated=true.
const ListMaxPages = 25

var validRegion = regexp.MustCompile(`^[a-z0-9-]+$`)

// Auth carries the API-signing-key material plus the compartment scope. The parsed
// ConfigurationProvider owns the request signer and backs both service clients.
type Auth struct {
	TenancyOCID     string
	UserOCID        string
	Region          string
	Fingerprint     string
	privateKey      string // PEM — never echoed (see redact)
	passphrase      string
	CompartmentOCID string
	provider        common.ConfigurationProvider
}

// GetAuth reads the standard credential block and builds the signing-key
// ConfigurationProvider, validating the host-selecting region and eagerly parsing
// the PEM so a bad key fails cleanly up front.
func GetAuth(inputs []*coreflow.Connection) (Auth, error) {
	a := Auth{
		TenancyOCID:     strings.TrimSpace(OptionalString(InputTenancyOCID, inputs)),
		UserOCID:        strings.TrimSpace(OptionalString(InputUserOCID, inputs)),
		Region:          strings.ToLower(strings.TrimSpace(OptionalString(InputRegion, inputs))),
		Fingerprint:     strings.TrimSpace(OptionalString(InputFingerprint, inputs)),
		privateKey:      OptionalString(InputPrivateKey, inputs),
		passphrase:      OptionalString(InputPassphrase, inputs),
		CompartmentOCID: strings.TrimSpace(OptionalString(InputCompartmentOCID, inputs)),
	}
	for field, val := range map[string]string{
		"tenancy OCID": a.TenancyOCID,
		"user OCID":    a.UserOCID,
		"region":       a.Region,
		"fingerprint":  a.Fingerprint,
	} {
		if val == "" {
			return Auth{}, fmt.Errorf("%s is required", field)
		}
	}
	if !validRegion.MatchString(a.Region) {
		return Auth{}, fmt.Errorf("region %q is not a valid OCI region (expected a plain identifier like uk-london-1)", a.Region)
	}
	if strings.TrimSpace(a.privateKey) == "" {
		return Auth{}, fmt.Errorf("private key (PEM) is required")
	}
	var pass *string
	if a.passphrase != "" {
		pass = &a.passphrase
	}
	a.provider = common.NewRawConfigurationProvider(a.TenancyOCID, a.UserOCID, a.Region, a.Fingerprint, a.privateKey, pass)
	if _, err := a.provider.PrivateRSAKey(); err != nil {
		return Auth{}, fmt.Errorf("private key could not be parsed — check it is the full PEM (and the passphrase, if set): %s", a.redact(err.Error()))
	}
	return a, nil
}

// BlockstorageClient owns volume / backup / group / policy CRUD.
func (a Auth) BlockstorageClient() (core.BlockstorageClient, error) {
	return core.NewBlockstorageClientWithConfigurationProvider(a.provider)
}

// ComputeClient owns attach / detach and the attachment reads.
func (a Auth) ComputeClient() (core.ComputeClient, error) {
	return core.NewComputeClientWithConfigurationProvider(a.provider)
}

// VolumeResourceClient reads the credential block + one Blockstorage-resource OCID
// (named by ocidInputName, e.g. "volume_ocid", "volume_backup_ocid") and builds the
// Blockstorage client — the shared preamble for volume/backup/group/policy per-
// resource actions. On any setup error it returns a ready ErrorResult.
func VolumeResourceClient(inputs []*coreflow.Connection, ocidInputName string) (auth Auth, client core.BlockstorageClient, ocid string, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, core.BlockstorageClient{}, "", ErrorResult(err.Error())
	}
	id, err := RequiredString(ocidInputName, inputs)
	if err != nil {
		return Auth{}, core.BlockstorageClient{}, "", ErrorResult(err.Error())
	}
	c, err := a.BlockstorageClient()
	if err != nil {
		return Auth{}, core.BlockstorageClient{}, "", ErrorResult(a.OCIError(err))
	}
	return a, c, id, nil
}

// ComputeResourceClient is the counterpart preamble for the attach/detach surface,
// which lives on the Compute client. ocidInputName names the path OCID the action
// keys on ("instance_ocid" for attach, "volume_attachment_ocid" for detach/get).
func ComputeResourceClient(inputs []*coreflow.Connection, ocidInputName string) (auth Auth, client core.ComputeClient, ocid string, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, core.ComputeClient{}, "", ErrorResult(err.Error())
	}
	id, err := RequiredString(ocidInputName, inputs)
	if err != nil {
		return Auth{}, core.ComputeClient{}, "", ErrorResult(err.Error())
	}
	c, err := a.ComputeClient()
	if err != nil {
		return Auth{}, core.ComputeClient{}, "", ErrorResult(a.OCIError(err))
	}
	return a, c, id, nil
}

func (a Auth) RequiredCompartment() (string, error) {
	if a.CompartmentOCID == "" {
		return "", fmt.Errorf("compartment OCID is required")
	}
	return a.CompartmentOCID, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

func OptionalString(name string, inputs []*coreflow.Connection) string {
	c := coreflow.FindConnection(name, inputs)
	if c == nil {
		return ""
	}
	if s := c.String(); s != nil {
		return *s
	}
	return ""
}

func fieldLabel(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "_", " "), "ocid", "OCID")
}

func RequiredString(name string, inputs []*coreflow.Connection) (string, error) {
	if v := strings.TrimSpace(OptionalString(name, inputs)); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s is required", fieldLabel(name))
}

func OptionalBool(name string, inputs []*coreflow.Connection, def bool) bool {
	c := coreflow.FindConnection(name, inputs)
	if c == nil {
		return def
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	return def
}

// BoolWasSet reports whether the operator provided any value for a boolean input.
func BoolWasSet(name string, inputs []*coreflow.Connection) bool {
	return strings.TrimSpace(OptionalString(name, inputs)) != ""
}

// OptionalInt64 reads a whole-number input into an *int64 (block-volume sizes and
// vpusPerGB are int64), returning (value, true) when set, (0, false) when blank, or
// an error when present but not an integer.
func OptionalInt64(name string, inputs []*coreflow.Connection) (int64, bool, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return 0, false, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be a whole number", fieldLabel(name))
	}
	return n, true, nil
}

// InputStrings splits a comma-separated input into a trimmed, non-empty slice.
func InputStrings(name string, inputs []*coreflow.Connection) []string {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// FreeformTags parses a JSON object input into map[string]string; blank → nil.
func FreeformTags(name string, inputs []*coreflow.Connection) (map[string]string, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var flat map[string]string
	if err := json.Unmarshal([]byte(raw), &flat); err != nil {
		return nil, fmt.Errorf("tags must be a JSON object of string values, e.g. {\"env\":\"prod\"}: %s", err.Error())
	}
	return flat, nil
}

func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func StringPtr(s string) *string { return &s }

func FormatTime(t *common.SDKTime) string {
	if t == nil {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// NormaliseRegion trims and lower-cases an operator-entered OCI region identifier.
// OCI region keys are canonically lower-case (uk-london-1, us-ashburn-1); the auth
// region is host-selecting, so GetAuth already lower-cases + guards it. The
// cross-region *destination* fields (backup copies, backup-policy replication) are
// data rather than host-selecting, but we normalise operator free-text the same way
// so a stray "US-ASHBURN-1" doesn't bounce off OCI as an invalid region. Centralised
// here so every destination-region field is handled identically and the intent is
// documented in one place.
func NormaliseRegion(name string, inputs []*coreflow.Connection) string {
	return strings.ToLower(strings.TrimSpace(OptionalString(name, inputs)))
}

// ---------------------------------------------------------------------------
// Resource summarisers — one per resource type so the output shape is identical
// across a resource's create/get/get_all actions.
// ---------------------------------------------------------------------------

func SummariseVolume(v *core.Volume) map[string]interface{} {
	m := map[string]interface{}{
		"id":                  Str(v.Id),
		"display_name":        Str(v.DisplayName),
		"availability_domain": Str(v.AvailabilityDomain),
		"compartment_id":      Str(v.CompartmentId),
		"lifecycle_state":     string(v.LifecycleState),
		"time_created":        FormatTime(v.TimeCreated),
		"is_hydrated":         v.IsHydrated != nil && *v.IsHydrated,
		"kms_key_id":          Str(v.KmsKeyId),
	}
	if v.SizeInGBs != nil {
		m["size_in_gbs"] = *v.SizeInGBs
	}
	if v.VpusPerGB != nil {
		m["vpus_per_gb"] = *v.VpusPerGB
	}
	return m
}

func SummariseBootVolume(v *core.BootVolume) map[string]interface{} {
	m := map[string]interface{}{
		"id":                  Str(v.Id),
		"display_name":        Str(v.DisplayName),
		"availability_domain": Str(v.AvailabilityDomain),
		"compartment_id":      Str(v.CompartmentId),
		"lifecycle_state":     string(v.LifecycleState),
		"time_created":        FormatTime(v.TimeCreated),
		"is_hydrated":         v.IsHydrated != nil && *v.IsHydrated,
		"kms_key_id":          Str(v.KmsKeyId),
	}
	if v.SizeInGBs != nil {
		m["size_in_gbs"] = *v.SizeInGBs
	}
	if v.VpusPerGB != nil {
		m["vpus_per_gb"] = *v.VpusPerGB
	}
	return m
}

func SummariseVolumeBackup(b *core.VolumeBackup) map[string]interface{} {
	m := map[string]interface{}{
		"id":              Str(b.Id),
		"display_name":    Str(b.DisplayName),
		"volume_id":       Str(b.VolumeId),
		"compartment_id":  Str(b.CompartmentId),
		"lifecycle_state": string(b.LifecycleState),
		"type":            string(b.Type),
		"source_type":     string(b.SourceType),
		"time_created":    FormatTime(b.TimeCreated),
	}
	if b.SizeInGBs != nil {
		m["size_in_gbs"] = *b.SizeInGBs
	}
	if b.UniqueSizeInGBs != nil {
		m["unique_size_in_gbs"] = *b.UniqueSizeInGBs
	}
	return m
}

func SummariseBootVolumeBackup(b *core.BootVolumeBackup) map[string]interface{} {
	m := map[string]interface{}{
		"id":              Str(b.Id),
		"display_name":    Str(b.DisplayName),
		"boot_volume_id":  Str(b.BootVolumeId),
		"compartment_id":  Str(b.CompartmentId),
		"lifecycle_state": string(b.LifecycleState),
		"type":            string(b.Type),
		"source_type":     string(b.SourceType),
		"time_created":    FormatTime(b.TimeCreated),
	}
	if b.SizeInGBs != nil {
		m["size_in_gbs"] = *b.SizeInGBs
	}
	if b.UniqueSizeInGBs != nil {
		m["unique_size_in_gbs"] = *b.UniqueSizeInGBs
	}
	return m
}

func SummariseVolumeGroup(g *core.VolumeGroup) map[string]interface{} {
	m := map[string]interface{}{
		"id":                  Str(g.Id),
		"display_name":        Str(g.DisplayName),
		"availability_domain": Str(g.AvailabilityDomain),
		"compartment_id":      Str(g.CompartmentId),
		"lifecycle_state":     string(g.LifecycleState),
		"volume_ids":          g.VolumeIds,
		"time_created":        FormatTime(g.TimeCreated),
	}
	if g.SizeInGBs != nil {
		m["size_in_gbs"] = *g.SizeInGBs
	}
	return m
}

func SummariseBackupPolicy(p *core.VolumeBackupPolicy) map[string]interface{} {
	return map[string]interface{}{
		"id":                Str(p.Id),
		"display_name":      Str(p.DisplayName),
		"compartment_id":    Str(p.CompartmentId),
		"schedules":         p.Schedules,
		"destination_region": Str(p.DestinationRegion),
		"time_created":      FormatTime(p.TimeCreated),
	}
}

func SummariseAssignment(a *core.VolumeBackupPolicyAssignment) map[string]interface{} {
	return map[string]interface{}{
		"id":           Str(a.Id),
		"asset_id":     Str(a.AssetId),
		"policy_id":    Str(a.PolicyId),
		"time_created": FormatTime(a.TimeCreated),
	}
}

// SummariseVolumeAttachment flattens the polymorphic VolumeAttachment interface via
// its base getter methods (the concrete type is iSCSI/paravirtualized/emulated).
func SummariseVolumeAttachment(a core.VolumeAttachment) map[string]interface{} {
	if a == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":                  Str(a.GetId()),
		"display_name":        Str(a.GetDisplayName()),
		"instance_id":         Str(a.GetInstanceId()),
		"volume_id":           Str(a.GetVolumeId()),
		"availability_domain": Str(a.GetAvailabilityDomain()),
		"lifecycle_state":     string(a.GetLifecycleState()),
		"time_created":        FormatTime(a.GetTimeCreated()),
	}
}

// SummariseBootVolumeAttachment flattens the concrete BootVolumeAttachment struct
// (boot-volume attachments are not polymorphic — unlike data-volume attachments).
func SummariseBootVolumeAttachment(a *core.BootVolumeAttachment) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(a.Id),
		"display_name":        Str(a.DisplayName),
		"instance_id":         Str(a.InstanceId),
		"boot_volume_id":      Str(a.BootVolumeId),
		"availability_domain": Str(a.AvailabilityDomain),
		"lifecycle_state":     string(a.LifecycleState),
		"time_created":        FormatTime(a.TimeCreated),
	}
}

func SummariseVolumeGroupBackup(b *core.VolumeGroupBackup) map[string]interface{} {
	m := map[string]interface{}{
		"id":                Str(b.Id),
		"display_name":      Str(b.DisplayName),
		"volume_group_id":   Str(b.VolumeGroupId),
		"compartment_id":    Str(b.CompartmentId),
		"lifecycle_state":   string(b.LifecycleState),
		"type":              string(b.Type),
		"source_type":       string(b.SourceType),
		"volume_backup_ids": b.VolumeBackupIds,
		"time_created":      FormatTime(b.TimeCreated),
	}
	if b.SizeInGBs != nil {
		m["size_in_gbs"] = *b.SizeInGBs
	}
	if b.UniqueSizeInGbs != nil {
		m["unique_size_in_gbs"] = *b.UniqueSizeInGbs
	}
	return m
}

func SummariseBlockVolumeReplica(r *core.BlockVolumeReplica) map[string]interface{} {
	m := map[string]interface{}{
		"id":                  Str(r.Id),
		"display_name":        Str(r.DisplayName),
		"block_volume_id":     Str(r.BlockVolumeId),
		"availability_domain": Str(r.AvailabilityDomain),
		"compartment_id":      Str(r.CompartmentId),
		"lifecycle_state":     string(r.LifecycleState),
		"time_created":        FormatTime(r.TimeCreated),
		"time_last_synced":    FormatTime(r.TimeLastSynced),
	}
	if r.SizeInGBs != nil {
		m["size_in_gbs"] = *r.SizeInGBs
	}
	return m
}

func SummariseBootVolumeReplica(r *core.BootVolumeReplica) map[string]interface{} {
	m := map[string]interface{}{
		"id":                  Str(r.Id),
		"display_name":        Str(r.DisplayName),
		"boot_volume_id":      Str(r.BootVolumeId),
		"availability_domain": Str(r.AvailabilityDomain),
		"compartment_id":      Str(r.CompartmentId),
		"lifecycle_state":     string(r.LifecycleState),
		"image_id":            Str(r.ImageId),
		"time_created":        FormatTime(r.TimeCreated),
		"time_last_synced":    FormatTime(r.TimeLastSynced),
	}
	if r.SizeInGBs != nil {
		m["size_in_gbs"] = *r.SizeInGBs
	}
	return m
}

func SummariseVolumeGroupReplica(r *core.VolumeGroupReplica) map[string]interface{} {
	m := map[string]interface{}{
		"id":                  Str(r.Id),
		"display_name":        Str(r.DisplayName),
		"volume_group_id":     Str(r.VolumeGroupId),
		"availability_domain": Str(r.AvailabilityDomain),
		"compartment_id":      Str(r.CompartmentId),
		"lifecycle_state":     string(r.LifecycleState),
		"time_created":        FormatTime(r.TimeCreated),
		"time_last_synced":    FormatTime(r.TimeLastSynced),
	}
	if r.SizeInGBs != nil {
		m["size_in_gbs"] = *r.SizeInGBs
	}
	return m
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{"success": false, "error": msg, "tool_result": msg}
}

func ServiceErrorCode(err error) (string, int) {
	if se, ok := common.IsServiceError(err); ok {
		return se.GetCode(), se.GetHTTPStatusCode()
	}
	return "", 0
}

func (a Auth) OCIError(err error) string {
	if err == nil {
		return ""
	}
	if se, ok := common.IsServiceError(err); ok {
		code := se.GetCode()
		if code == "" {
			code = fmt.Sprintf("HTTP %d", se.GetHTTPStatusCode())
		}
		return a.redact(fmt.Sprintf("OCI rejected the request (%s): %s", code, se.GetMessage()))
	}
	return a.redact(firstLine(err.Error()))
}

func (a Auth) redact(s string) string {
	if k := strings.TrimSpace(a.privateKey); k != "" {
		s = strings.ReplaceAll(s, k, "REDACTED")
		s = strings.ReplaceAll(s, a.privateKey, "REDACTED")
	}
	if a.passphrase != "" {
		s = strings.ReplaceAll(s, a.passphrase, "REDACTED")
	}
	return s
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// Context is what every action passes: no request deadline from the executor.
func Context() context.Context { return context.Background() }

// Package filestorage holds what every Oracle Cloud (OCI) File Storage action shares:
// the API-signing-key credential, the FileStorageClient factory, the two client
// preambles, the resource summarisers, and the input/result helpers. Like the sibling
// OCI packages it has no Execute function, so the manifest generator skips it — but its
// category.go supplies the "File Storage" sub-group.
//
// FSS quirks the actions lean on: (1) file systems, mount targets and export sets are
// AVAILABILITY-DOMAIN-scoped — create and list both require an availability_domain
// (RequiredAvailabilityDomain); (2) an export links a FILE SYSTEM to an EXPORT SET (which
// belongs to a mount target) at a PATH; (3) the API is largely SYNCHRONOUS — resources
// move through lifecycle states (CREATING→ACTIVE, DELETING→DELETED), so there is no
// work-request model. Poll Get <resource> until ACTIVE / 404.
package filestorage

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/filestorage"

	coreflow "flomation.app/automate/executor"
)

const (
	InputTenancyOCID        = "tenancy_ocid"
	InputUserOCID           = "user_ocid"
	InputRegion             = "region"
	InputFingerprint        = "fingerprint"
	InputPrivateKey         = "private_key"
	InputPassphrase         = "private_key_passphrase"
	InputCompartmentOCID    = "compartment_ocid"
	InputAvailabilityDomain = "availability_domain"
)

// ListMaxPages bounds every list action's pagination walk.
const ListMaxPages = 25

var validRegion = regexp.MustCompile(`^[a-z0-9-]+$`)

// Auth carries the API-signing-key material plus the compartment scope.
type Auth struct {
	TenancyOCID     string
	UserOCID        string
	Region          string
	Fingerprint     string
	privateKey      string
	passphrase      string
	CompartmentOCID string
	provider        common.ConfigurationProvider
}

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
		"tenancy OCID": a.TenancyOCID, "user OCID": a.UserOCID, "region": a.Region, "fingerprint": a.Fingerprint,
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

func (a Auth) FileStorageClient() (filestorage.FileStorageClient, error) {
	return filestorage.NewFileStorageClientWithConfigurationProvider(a.provider)
}

// Client is the preamble for compartment-scoped ops (create, list).
func Client(inputs []*coreflow.Connection) (auth Auth, client filestorage.FileStorageClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, filestorage.FileStorageClient{}, ErrorResult(err.Error())
	}
	c, err := a.FileStorageClient()
	if err != nil {
		return Auth{}, filestorage.FileStorageClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// ResourceClient additionally reads one resource identifier (named by inputName).
func ResourceClient(inputs []*coreflow.Connection, inputName string) (auth Auth, client filestorage.FileStorageClient, id string, errResult map[string]interface{}) {
	a, c, errRes := Client(inputs)
	if errRes != nil {
		return Auth{}, filestorage.FileStorageClient{}, "", errRes
	}
	v, err := RequiredString(inputName, inputs)
	if err != nil {
		return Auth{}, filestorage.FileStorageClient{}, "", ErrorResult(err.Error())
	}
	return a, c, v, nil
}

func (a Auth) RequiredCompartment() (string, error) {
	if a.CompartmentOCID == "" {
		return "", fmt.Errorf("compartment OCID is required")
	}
	return a.CompartmentOCID, nil
}

// RequiredAvailabilityDomain reads the AD input — mandatory for file systems, mount
// targets and export sets on create and list.
func RequiredAvailabilityDomain(inputs []*coreflow.Connection) (string, error) {
	return RequiredString(InputAvailabilityDomain, inputs)
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

func BoolWasSet(name string, inputs []*coreflow.Connection) bool {
	return strings.TrimSpace(OptionalString(name, inputs)) != ""
}

func OptionalInt(name string, inputs []*coreflow.Connection) (int, bool, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return 0, false, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be a whole number", fieldLabel(name))
	}
	return n, true, nil
}

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

func Int64OrNil(p *int64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func FormatTime(t *common.SDKTime) string {
	if t == nil {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// Resource summarisers (core resources; the long-tail types build maps inline)
// ---------------------------------------------------------------------------

func SummariseFileSystem(f *filestorage.FileSystem) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(f.Id),
		"display_name":        Str(f.DisplayName),
		"compartment_id":      Str(f.CompartmentId),
		"availability_domain": Str(f.AvailabilityDomain),
		"lifecycle_state":     string(f.LifecycleState),
		"metered_bytes":       Int64OrNil(f.MeteredBytes),
		"kms_key_id":          Str(f.KmsKeyId),
		"time_created":        FormatTime(f.TimeCreated),
	}
}

func SummariseFileSystemSummary(f *filestorage.FileSystemSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(f.Id),
		"display_name":        Str(f.DisplayName),
		"compartment_id":      Str(f.CompartmentId),
		"availability_domain": Str(f.AvailabilityDomain),
		"lifecycle_state":     string(f.LifecycleState),
		"metered_bytes":       Int64OrNil(f.MeteredBytes),
		"time_created":        FormatTime(f.TimeCreated),
	}
}

func SummariseMountTarget(m *filestorage.MountTarget) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(m.Id),
		"display_name":        Str(m.DisplayName),
		"compartment_id":      Str(m.CompartmentId),
		"availability_domain": Str(m.AvailabilityDomain),
		"subnet_id":           Str(m.SubnetId),
		"export_set_id":       Str(m.ExportSetId),
		"private_ip_ids":      m.PrivateIpIds,
		"lifecycle_state":     string(m.LifecycleState),
		"time_created":        FormatTime(m.TimeCreated),
	}
}

func SummariseMountTargetSummary(m *filestorage.MountTargetSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(m.Id),
		"display_name":        Str(m.DisplayName),
		"compartment_id":      Str(m.CompartmentId),
		"availability_domain": Str(m.AvailabilityDomain),
		"subnet_id":           Str(m.SubnetId),
		"export_set_id":       Str(m.ExportSetId),
		"lifecycle_state":     string(m.LifecycleState),
		"time_created":        FormatTime(m.TimeCreated),
	}
}

func SummariseExport(e *filestorage.Export) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(e.Id),
		"export_set_id":   Str(e.ExportSetId),
		"file_system_id":  Str(e.FileSystemId),
		"path":            Str(e.Path),
		"lifecycle_state": string(e.LifecycleState),
		"time_created":    FormatTime(e.TimeCreated),
	}
}

func SummariseExportSummary(e *filestorage.ExportSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(e.Id),
		"export_set_id":   Str(e.ExportSetId),
		"file_system_id":  Str(e.FileSystemId),
		"path":            Str(e.Path),
		"lifecycle_state": string(e.LifecycleState),
		"time_created":    FormatTime(e.TimeCreated),
	}
}

func SummariseExportSet(e *filestorage.ExportSet) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(e.Id),
		"display_name":        Str(e.DisplayName),
		"compartment_id":      Str(e.CompartmentId),
		"availability_domain": Str(e.AvailabilityDomain),
		"vcn_id":              Str(e.VcnId),
		"lifecycle_state":     string(e.LifecycleState),
		"max_fs_stat_bytes":   Int64OrNil(e.MaxFsStatBytes),
		"max_fs_stat_files":   Int64OrNil(e.MaxFsStatFiles),
		"time_created":        FormatTime(e.TimeCreated),
	}
}

func SummariseSnapshot(s *filestorage.Snapshot) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(s.Id),
		"name":            Str(s.Name),
		"file_system_id":  Str(s.FileSystemId),
		"snapshot_type":   string(s.SnapshotType),
		"lifecycle_state": string(s.LifecycleState),
		"time_created":    FormatTime(s.TimeCreated),
		"snapshot_time":   FormatTime(s.SnapshotTime),
	}
}

func SummariseSnapshotSummary(s *filestorage.SnapshotSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(s.Id),
		"name":            Str(s.Name),
		"file_system_id":  Str(s.FileSystemId),
		"snapshot_type":   string(s.SnapshotType),
		"lifecycle_state": string(s.LifecycleState),
		"time_created":    FormatTime(s.TimeCreated),
	}
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

// Result is the standard success envelope: tool_result + success plus any extra keys.
func Result(msg string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{"tool_result": msg, "success": true}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{"success": false, "error": msg, "tool_result": msg}
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

func Context() context.Context { return context.Background() }

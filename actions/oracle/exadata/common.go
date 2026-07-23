// Package exadata holds what every Oracle Cloud (OCI) Exadata Database Service on
// Dedicated Infrastructure action shares: the API-signing-key credential, the Database
// service client factory, the resource summarisers, and the input/result helpers. Like the
// sibling OCI packages it has no Execute function, so the manifest generator skips it — but
// its category.go supplies the "Exadata" sub-group.
//
// This node targets the cloud-native Exadata Database Service (ExaDB-D): CloudExadata-
// Infrastructure (the rack), CloudVmCluster (the cluster on it), DbServers/DbNodes, and
// MaintenanceRuns. It shares the single database.DatabaseClient with the Autonomous
// Database node. Create/update/delete are asynchronous — they return the resource in a
// PROVISIONING/UPDATING/… state plus a work-request id; poll the Get action until the
// lifecycle state settles.
package exadata

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/database"

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

func (a Auth) RequiredCompartment() (string, error) {
	if a.CompartmentOCID == "" {
		return "", fmt.Errorf("compartment OCID is required")
	}
	return a.CompartmentOCID, nil
}

// Client builds the single Database service client the whole node uses.
func Client(inputs []*coreflow.Connection) (auth Auth, client database.DatabaseClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, database.DatabaseClient{}, ErrorResult(err.Error())
	}
	c, err := database.NewDatabaseClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, database.DatabaseClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
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

func fieldLabel(name string) string { return strings.ReplaceAll(name, "_", " ") }

func RequiredString(name string, inputs []*coreflow.Connection) (string, error) {
	v := strings.TrimSpace(OptionalString(name, inputs))
	if v == "" {
		return "", fmt.Errorf("%s is required", fieldLabel(name))
	}
	return v, nil
}

func OptionalBool(name string, inputs []*coreflow.Connection, def bool) bool {
	c := coreflow.FindConnection(name, inputs)
	if c == nil {
		return def
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	raw := strings.ToLower(strings.TrimSpace(OptionalString(name, inputs)))
	if raw == "" {
		return def
	}
	return raw == "true" || raw == "yes" || raw == "1"
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

func RequiredInt(name string, inputs []*coreflow.Connection) (int, error) {
	n, ok, err := OptionalInt(name, inputs)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("%s is required", fieldLabel(name))
	}
	return n, nil
}

// InputStrings splits a comma-separated input into a trimmed, non-empty slice.
func InputStrings(name string, inputs []*coreflow.Connection) []string {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
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

func IntOrNil(p *int) interface{} {
	if p == nil {
		return nil
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
// Resource summarisers
// ---------------------------------------------------------------------------

func SummariseCloudExadataInfrastructure(x *database.CloudExadataInfrastructure) map[string]interface{} {
	return map[string]interface{}{
		"id":                    Str(x.Id),
		"display_name":          Str(x.DisplayName),
		"compartment_id":        Str(x.CompartmentId),
		"shape":                 Str(x.Shape),
		"availability_domain":   Str(x.AvailabilityDomain),
		"lifecycle_state":       string(x.LifecycleState),
		"compute_count":         IntOrNil(x.ComputeCount),
		"storage_count":         IntOrNil(x.StorageCount),
		"total_storage_size_gb": IntOrNil(x.TotalStorageSizeInGBs),
		"cpu_count":             IntOrNil(x.CpuCount),
		"memory_size_gb":        IntOrNil(x.MemorySizeInGBs),
	}
}

func SummariseCloudExadataInfrastructureSummary(x *database.CloudExadataInfrastructureSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                    Str(x.Id),
		"display_name":          Str(x.DisplayName),
		"compartment_id":        Str(x.CompartmentId),
		"shape":                 Str(x.Shape),
		"availability_domain":   Str(x.AvailabilityDomain),
		"lifecycle_state":       string(x.LifecycleState),
		"compute_count":         IntOrNil(x.ComputeCount),
		"storage_count":         IntOrNil(x.StorageCount),
		"total_storage_size_gb": IntOrNil(x.TotalStorageSizeInGBs),
	}
}

func SummariseCloudVmCluster(v *database.CloudVmCluster) map[string]interface{} {
	return map[string]interface{}{
		"id":                              Str(v.Id),
		"display_name":                    Str(v.DisplayName),
		"compartment_id":                  Str(v.CompartmentId),
		"cloud_exadata_infrastructure_id": Str(v.CloudExadataInfrastructureId),
		"shape":                           Str(v.Shape),
		"availability_domain":             Str(v.AvailabilityDomain),
		"subnet_id":                       Str(v.SubnetId),
		"hostname":                        Str(v.Hostname),
		"domain":                          Str(v.Domain),
		"cpu_core_count":                  IntOrNil(v.CpuCoreCount),
		"listener_port":                   Int64OrNil(v.ListenerPort),
		"lifecycle_state":                 string(v.LifecycleState),
	}
}

func SummariseCloudVmClusterSummary(v *database.CloudVmClusterSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                              Str(v.Id),
		"display_name":                    Str(v.DisplayName),
		"compartment_id":                  Str(v.CompartmentId),
		"cloud_exadata_infrastructure_id": Str(v.CloudExadataInfrastructureId),
		"shape":                           Str(v.Shape),
		"availability_domain":             Str(v.AvailabilityDomain),
		"hostname":                        Str(v.Hostname),
		"cpu_core_count":                  IntOrNil(v.CpuCoreCount),
		"lifecycle_state":                 string(v.LifecycleState),
	}
}

func SummariseDbServer(d *database.DbServer) map[string]interface{} {
	return map[string]interface{}{
		"id":                        Str(d.Id),
		"display_name":              Str(d.DisplayName),
		"compartment_id":            Str(d.CompartmentId),
		"exadata_infrastructure_id": Str(d.ExadataInfrastructureId),
		"shape":                     Str(d.Shape),
		"cpu_core_count":            IntOrNil(d.CpuCoreCount),
		"memory_size_gb":            IntOrNil(d.MemorySizeInGBs),
		"lifecycle_state":           string(d.LifecycleState),
	}
}

func SummariseDbServerSummary(d *database.DbServerSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                        Str(d.Id),
		"display_name":              Str(d.DisplayName),
		"compartment_id":            Str(d.CompartmentId),
		"exadata_infrastructure_id": Str(d.ExadataInfrastructureId),
		"shape":                     Str(d.Shape),
		"cpu_core_count":            IntOrNil(d.CpuCoreCount),
		"lifecycle_state":           string(d.LifecycleState),
	}
}

func SummariseDbNode(n *database.DbNode) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(n.Id),
		"hostname":            Str(n.Hostname),
		"db_system_id":        Str(n.DbSystemId),
		"fault_domain":        Str(n.FaultDomain),
		"lifecycle_state":     string(n.LifecycleState),
		"software_storage_gb": IntOrNil(n.SoftwareStorageSizeInGB),
	}
}

func SummariseDbNodeSummary(n *database.DbNodeSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(n.Id),
		"hostname":        Str(n.Hostname),
		"db_system_id":    Str(n.DbSystemId),
		"fault_domain":    Str(n.FaultDomain),
		"lifecycle_state": string(n.LifecycleState),
	}
}

func SummariseMaintenanceRun(m *database.MaintenanceRun) map[string]interface{} {
	return map[string]interface{}{
		"id":                   Str(m.Id),
		"display_name":         Str(m.DisplayName),
		"compartment_id":       Str(m.CompartmentId),
		"lifecycle_state":      string(m.LifecycleState),
		"maintenance_type":     string(m.MaintenanceType),
		"maintenance_subtype":  string(m.MaintenanceSubtype),
		"target_resource_type": string(m.TargetResourceType),
		"target_resource_id":   Str(m.TargetResourceId),
		"patching_mode":        string(m.PatchingMode),
		"time_scheduled":       FormatTime(m.TimeScheduled),
		"time_started":         FormatTime(m.TimeStarted),
		"time_ended":           FormatTime(m.TimeEnded),
	}
}

func SummariseMaintenanceRunSummary(m *database.MaintenanceRunSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                   Str(m.Id),
		"display_name":         Str(m.DisplayName),
		"compartment_id":       Str(m.CompartmentId),
		"lifecycle_state":      string(m.LifecycleState),
		"maintenance_type":     string(m.MaintenanceType),
		"target_resource_type": string(m.TargetResourceType),
		"target_resource_id":   Str(m.TargetResourceId),
		"time_scheduled":       FormatTime(m.TimeScheduled),
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

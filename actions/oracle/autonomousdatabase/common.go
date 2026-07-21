// Package autonomousdatabase holds what every Oracle Cloud (OCI) Autonomous
// Database action shares: the API-signing-key credential, the database service
// client factory, the per-database client preamble, and the input/result helpers.
// Like oracle/compute and oracle/objectstorage it has no Execute function, so the
// manifest generator skips it — but its category.go supplies the "Autonomous
// Database" sub-category.
//
// The auth block is the OCI signing-key model, identical to the other two OCI
// nodes (tenancy/user OCID, region, fingerprint, private-key PEM, optional
// passphrase). The only service-specific piece is DatabaseClient(): every request
// is signed by the shared ConfigurationProvider, and the database.DatabaseClient
// reads its region + signer from it. As with the sibling packages, the manifest
// generator only resolves INLINE Inputs literals, so the credential + compartment
// input *declarations* must still be copy-pasted into each action's Inputs — only
// the Execute-side logic is shared here.
package autonomousdatabase

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

// Standard input names shared by every OCI Autonomous Database action.
const (
	InputTenancyOCID     = "tenancy_ocid"
	InputUserOCID        = "user_ocid"
	InputRegion          = "region"
	InputFingerprint     = "fingerprint"
	InputPrivateKey      = "private_key"
	InputPassphrase      = "private_key_passphrase"
	InputCompartmentOCID = "compartment_ocid"
	InputDatabaseOCID    = "autonomous_database_id"
)

// ListMaxPages bounds every list action's pagination walk so one node run can't
// turn into an unbounded sequence of API calls; a walk that hits the cap sets
// truncated=true so a capped result is distinguishable from a complete one.
const ListMaxPages = 25

// validRegion constrains the host-selecting region to a plain label: the SDK
// builds https://<service>.<region>.<realm> from it, and a region containing a dot
// short-circuits the realm suffix so a signed request could be redirected to an
// arbitrary host. Legitimate OCI regions never contain a dot. See GetAuth.
var validRegion = regexp.MustCompile(`^[a-z0-9-]+$`)

// Auth carries the API-signing-key material plus the compartment scope that
// list/create calls need. Per-database ops are scoped by the database OCID.
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
// ConfigurationProvider, validating the region (host-selecting) and eagerly
// parsing the PEM so a bad key fails with a clean message up front.
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

// DatabaseClient builds an authenticated OCI Database client (the service that
// owns Autonomous Database).
func (a Auth) DatabaseClient() (database.DatabaseClient, error) {
	return database.NewDatabaseClientWithConfigurationProvider(a.provider)
}

// PerDatabaseClient reads the credential block + the autonomous_database_id input
// and builds a Database client — the shared preamble for every per-database action
// (get / start / stop / restart / delete / update / scale / wallet / …). On any
// setup error it returns a ready ErrorResult so the caller can
// `return errResult, nil`; on success errResult is nil.
func PerDatabaseClient(inputs []*coreflow.Connection) (auth Auth, client database.DatabaseClient, dbOCID string, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, database.DatabaseClient{}, "", ErrorResult(err.Error())
	}
	id, err := RequiredString(InputDatabaseOCID, inputs)
	if err != nil {
		return Auth{}, database.DatabaseClient{}, "", ErrorResult(err.Error())
	}
	c, err := a.DatabaseClient()
	if err != nil {
		return Auth{}, database.DatabaseClient{}, "", ErrorResult(a.OCIError(err))
	}
	return a, c, id, nil
}

// RequiredCompartment names the field when a compartment-scoped action is missing it.
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

// fieldLabel turns an input name into an operator-facing label: underscores become
// spaces and the OCID token is upper-cased, so "autonomous_database_id" reads
// "autonomous database id" (consistent with RequiredCompartment's wording).
func fieldLabel(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "_", " "), "ocid", "OCID")
}

func RequiredString(name string, inputs []*coreflow.Connection) (string, error) {
	if v := strings.TrimSpace(OptionalString(name, inputs)); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s is required", fieldLabel(name))
}

// OptionalBool returns a boolean input's value, or def when absent/unset. Uses
// Connection.Boolean(), which resolves both literal checkboxes and variable-bound
// values (connection-accessor fix, executor !188).
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

// OptionalInt reads a whole-number input, returning (value, true) when set and
// parseable, (0, false) when blank, or an error when present but not an integer.
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

// OptionalFloat32 reads a decimal input (e.g. OCPU / ECPU compute count),
// returning (value, true) when set and parseable, (0, false) when blank, or an
// error when present but not a number.
func OptionalFloat32(name string, inputs []*coreflow.Connection) (float32, bool, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return 0, false, nil
	}
	f, err := strconv.ParseFloat(raw, 32)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be a number", fieldLabel(name))
	}
	return float32(f), true, nil
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

// StringMap parses a JSON object input into map[string]string; blank → nil. The
// label names the field in the error message (e.g. "tags").
func StringMap(name, label string, inputs []*coreflow.Connection) (map[string]string, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var flat map[string]string
	if err := json.Unmarshal([]byte(raw), &flat); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object of string values, e.g. {\"key\":\"value\"}: %s", label, err.Error())
	}
	return flat, nil
}

// FreeformTags parses a JSON object input into map[string]string; blank → nil.
func FreeformTags(name string, inputs []*coreflow.Connection) (map[string]string, error) {
	return StringMap(name, "tags", inputs)
}

func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func StringPtr(s string) *string { return &s }

// FormatTime renders an OCI SDKTime as RFC3339 (UTC) so every action emits
// timestamps in one machine-parseable format; blank for a nil time.
func FormatTime(t *common.SDKTime) string {
	if t == nil {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// SummariseAutonomousDatabase flattens the SDK model into a compact, JSON-friendly
// map. Shared by db_get, db_get_all and the lifecycle actions so the "database"
// output shape is identical everywhere. lifecycle_state is the OCI state
// (PROVISIONING / AVAILABLE / STOPPED / TERMINATED / …) an operator acts on.
func SummariseAutonomousDatabase(db *database.AutonomousDatabase) map[string]interface{} {
	m := map[string]interface{}{
		"id":                      Str(db.Id),
		"display_name":            Str(db.DisplayName),
		"db_name":                 Str(db.DbName),
		"lifecycle_state":         string(db.LifecycleState),
		"db_workload":             string(db.DbWorkload),
		"compute_model":           string(db.ComputeModel),
		"license_model":           string(db.LicenseModel),
		"db_version":              Str(db.DbVersion),
		"compartment_id":          Str(db.CompartmentId),
		"is_free_tier":            db.IsFreeTier != nil && *db.IsFreeTier,
		"is_auto_scaling_enabled": db.IsAutoScalingEnabled != nil && *db.IsAutoScalingEnabled,
	}
	if db.CpuCoreCount != nil {
		m["cpu_core_count"] = *db.CpuCoreCount
	}
	if db.OcpuCount != nil {
		m["ocpu_count"] = *db.OcpuCount
	}
	if db.ComputeCount != nil {
		m["compute_count"] = *db.ComputeCount
	}
	if db.DataStorageSizeInGBs != nil {
		m["data_storage_size_in_gbs"] = *db.DataStorageSizeInGBs
	}
	if db.DataStorageSizeInTBs != nil {
		m["data_storage_size_in_tbs"] = *db.DataStorageSizeInTBs
	}
	if db.TimeCreated != nil {
		m["time_created"] = FormatTime(db.TimeCreated)
	}
	tags := map[string]string{}
	for k, v := range db.FreeformTags {
		tags[k] = v
	}
	m["freeform_tags"] = tags
	return m
}

// SummariseAutonomousDatabaseSummary is the list-item counterpart of
// SummariseAutonomousDatabase: ListAutonomousDatabases returns the parallel
// AutonomousDatabaseSummary type (identical field names, distinct Go type), so the
// list action needs its own flattener to keep the "database" output shape byte-for-
// byte the same as db_get / the lifecycle actions.
func SummariseAutonomousDatabaseSummary(db *database.AutonomousDatabaseSummary) map[string]interface{} {
	m := map[string]interface{}{
		"id":                      Str(db.Id),
		"display_name":            Str(db.DisplayName),
		"db_name":                 Str(db.DbName),
		"lifecycle_state":         string(db.LifecycleState),
		"db_workload":             string(db.DbWorkload),
		"compute_model":           string(db.ComputeModel),
		"license_model":           string(db.LicenseModel),
		"db_version":              Str(db.DbVersion),
		"compartment_id":          Str(db.CompartmentId),
		"is_free_tier":            db.IsFreeTier != nil && *db.IsFreeTier,
		"is_auto_scaling_enabled": db.IsAutoScalingEnabled != nil && *db.IsAutoScalingEnabled,
	}
	if db.CpuCoreCount != nil {
		m["cpu_core_count"] = *db.CpuCoreCount
	}
	if db.OcpuCount != nil {
		m["ocpu_count"] = *db.OcpuCount
	}
	if db.ComputeCount != nil {
		m["compute_count"] = *db.ComputeCount
	}
	if db.DataStorageSizeInGBs != nil {
		m["data_storage_size_in_gbs"] = *db.DataStorageSizeInGBs
	}
	if db.DataStorageSizeInTBs != nil {
		m["data_storage_size_in_tbs"] = *db.DataStorageSizeInTBs
	}
	if db.TimeCreated != nil {
		m["time_created"] = FormatTime(db.TimeCreated)
	}
	tags := map[string]string{}
	for k, v := range db.FreeformTags {
		tags[k] = v
	}
	m["freeform_tags"] = tags
	return m
}

// SummariseBackup and SummariseBackupSummary flatten an Autonomous Database
// backup into a compact map. Like the database summarisers, the two exist because
// Get/Create return the full AutonomousDatabaseBackup while List returns the
// parallel AutonomousDatabaseBackupSummary — both MUST emit the SAME keys with the
// SAME presence rules so the "backup" output shape is identical across
// db_create_backup, db_get_backup and db_list_backups.
func SummariseBackup(b *database.AutonomousDatabaseBackup) map[string]interface{} {
	m := backupBase(Str(b.Id), Str(b.DisplayName), Str(b.AutonomousDatabaseId), Str(b.CompartmentId),
		string(b.Type), string(b.LifecycleState), Str(b.DbVersion), Str(b.Region), Str(b.LifecycleDetails),
		b.IsAutomatic, b.IsRestorable, b.RetentionPeriodInDays,
		FormatTime(b.TimeStarted), FormatTime(b.TimeEnded), FormatTime(b.TimeAvailableTill))
	if b.DatabaseSizeInTBs != nil {
		m["database_size_in_tbs"] = *b.DatabaseSizeInTBs
	}
	if b.SizeInTBs != nil {
		m["size_in_tbs"] = *b.SizeInTBs
	}
	return m
}

func SummariseBackupSummary(b *database.AutonomousDatabaseBackupSummary) map[string]interface{} {
	m := backupBase(Str(b.Id), Str(b.DisplayName), Str(b.AutonomousDatabaseId), Str(b.CompartmentId),
		string(b.Type), string(b.LifecycleState), Str(b.DbVersion), Str(b.Region), Str(b.LifecycleDetails),
		b.IsAutomatic, b.IsRestorable, b.RetentionPeriodInDays,
		FormatTime(b.TimeStarted), FormatTime(b.TimeEnded), FormatTime(b.TimeAvailableTill))
	if b.DatabaseSizeInTBs != nil {
		m["database_size_in_tbs"] = *b.DatabaseSizeInTBs
	}
	if b.SizeInTBs != nil {
		m["size_in_tbs"] = *b.SizeInTBs
	}
	return m
}

// backupBase builds the shared backup key set so both summarisers agree byte-for-
// byte. is_long_term_backup is derived from the type ("LONGTERM"); there is no
// dedicated bool on the model.
func backupBase(id, displayName, adbID, compartmentID, typ, lifecycleState, dbVersion, region, lifecycleDetails string,
	isAutomatic, isRestorable *bool, retentionDays *int, timeStarted, timeEnded, timeAvailableTill string) map[string]interface{} {
	m := map[string]interface{}{
		"id":                     id,
		"display_name":           displayName,
		"autonomous_database_id": adbID,
		"compartment_id":         compartmentID,
		"type":                   typ,
		"lifecycle_state":        lifecycleState,
		"is_long_term_backup":    typ == "LONGTERM",
		"is_automatic":           isAutomatic != nil && *isAutomatic,
		"is_restorable":          isRestorable != nil && *isRestorable,
		"db_version":             dbVersion,
		"region":                 region,
		"lifecycle_details":      lifecycleDetails,
		"time_started":           timeStarted,
		"time_ended":             timeEnded,
		"time_available_till":    timeAvailableTill,
	}
	if retentionDays != nil {
		m["retention_period_in_days"] = *retentionDays
	}
	return m
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{"success": false, "error": msg, "tool_result": msg}
}

// ServiceErrorCode returns the OCI service error code (e.g. "NotAuthorized") and
// HTTP status for err, or ("", 0) when err is not an OCI service error.
func ServiceErrorCode(err error) (string, int) {
	if se, ok := common.IsServiceError(err); ok {
		return se.GetCode(), se.GetHTTPStatusCode()
	}
	return "", 0
}

// OCIError summarises an OCI SDK error, redacting the key/passphrase defensively.
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

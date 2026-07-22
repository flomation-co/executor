// Package nosql holds what every Oracle Cloud (OCI) NoSQL Database action shares: the
// API-signing-key credential, the regional NosqlClient, the Table/Index summarisers, and the
// input/result helpers. Like the sibling OCI packages it has no Execute function, so the manifest
// generator skips it — but its category.go supplies the "NoSQL Database" sub-group.
//
// OCI NoSQL Database is a single regional service: it manages tables and their secondary indexes,
// reads/updates/deletes rows, and runs SQL queries against a fully managed key-value/JSON store.
package nosql

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/nosql"

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

// Client is the regional NoSQL Database client.
func Client(inputs []*coreflow.Connection) (auth Auth, client nosql.NosqlClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, nosql.NosqlClient{}, ErrorResult(err.Error())
	}
	c, err := nosql.NewNosqlClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, nosql.NosqlClient{}, ErrorResult(a.OCIError(err))
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

func OptionalInt(name string, inputs []*coreflow.Connection) (val int, ok bool, err error) {
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

// OptionalBoolPtr returns nil when the field is blank so a partial update leaves it unchanged.
func OptionalBoolPtr(name string, inputs []*coreflow.Connection) *bool {
	c := coreflow.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	if b := c.Boolean(); b != nil {
		return b
	}
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil
	}
	v := strings.EqualFold(raw, "true")
	return &v
}

func FreeformTags(name string, inputs []*coreflow.Connection) (map[string]string, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var flat map[string]string
	if err := json.Unmarshal([]byte(raw), &flat); err != nil {
		return nil, fmt.Errorf("freeform tags must be a JSON object of string values, e.g. {\"env\":\"prod\"}: %s", err.Error())
	}
	return flat, nil
}

func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func Bool(p *bool) interface{} {
	if p == nil {
		return nil
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

// summariseTableLimits flattens a table's throughput/storage configuration.
func summariseTableLimits(l *nosql.TableLimits) map[string]interface{} {
	if l == nil {
		return nil
	}
	return map[string]interface{}{
		"max_read_units":     IntOrNil(l.MaxReadUnits),
		"max_write_units":    IntOrNil(l.MaxWriteUnits),
		"max_storage_in_gbs": IntOrNil(l.MaxStorageInGBs),
		"capacity_mode":      string(l.CapacityMode),
	}
}

// summariseIndexKeys flattens the ordered key columns of a secondary index.
func summariseIndexKeys(keys []nosql.IndexKey) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(keys))
	for i := range keys {
		out = append(out, map[string]interface{}{
			"column_name":     Str(keys[i].ColumnName),
			"json_path":       Str(keys[i].JsonPath),
			"json_field_type": Str(keys[i].JsonFieldType),
		})
	}
	return out
}

func SummariseTable(r *nosql.Table) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(r.Id),
		"name":                Str(r.Name),
		"compartment_id":      Str(r.CompartmentId),
		"lifecycle_state":     string(r.LifecycleState),
		"lifecycle_details":   Str(r.LifecycleDetails),
		"table_limits":        summariseTableLimits(r.TableLimits),
		"ddl_statement":       Str(r.DdlStatement),
		"schema_state":        string(r.SchemaState),
		"is_auto_reclaimable": Bool(r.IsAutoReclaimable),
		"is_multi_region":     Bool(r.IsMultiRegion),
		"time_created":        FormatTime(r.TimeCreated),
		"time_updated":        FormatTime(r.TimeUpdated),
	}
}

func SummariseTableSummary(r *nosql.TableSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(r.Id),
		"name":                Str(r.Name),
		"compartment_id":      Str(r.CompartmentId),
		"lifecycle_state":     string(r.LifecycleState),
		"table_limits":        summariseTableLimits(r.TableLimits),
		"is_auto_reclaimable": Bool(r.IsAutoReclaimable),
		"is_multi_region":     Bool(r.IsMultiRegion),
		"time_created":        FormatTime(r.TimeCreated),
		"time_updated":        FormatTime(r.TimeUpdated),
	}
}

func SummariseIndex(r *nosql.Index) map[string]interface{} {
	return map[string]interface{}{
		"name":              Str(r.Name),
		"compartment_id":    Str(r.CompartmentId),
		"table_name":        Str(r.TableName),
		"table_id":          Str(r.TableId),
		"keys":              summariseIndexKeys(r.Keys),
		"lifecycle_state":   string(r.LifecycleState),
		"lifecycle_details": Str(r.LifecycleDetails),
	}
}

func SummariseIndexSummary(r *nosql.IndexSummary) map[string]interface{} {
	return map[string]interface{}{
		"name":              Str(r.Name),
		"keys":              summariseIndexKeys(r.Keys),
		"lifecycle_state":   string(r.LifecycleState),
		"lifecycle_details": Str(r.LifecycleDetails),
	}
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

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

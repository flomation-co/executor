// Package logging holds what every Oracle Cloud (OCI) Logging action shares: the API-signing-key
// credential, the two clients this service spans, the resource summarisers, and the input/result
// helpers. Like the sibling OCI packages it has no Execute function, so the manifest generator
// skips it — but its category.go supplies the "Logging" sub-group.
//
// OCI Logging has two planes on DIFFERENT clients:
//   - LoggingManagementClient — the REGIONAL control plane: log groups, logs (service and custom),
//     unified monitoring agent configurations, log saved searches, and work requests.
//   - LogSearchClient — the SEARCH plane: SearchLogs queries stored log content across the tenancy.
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/logging"
	"github.com/oracle/oci-go-sdk/v65/loggingsearch"

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

// MgmtClient is the regional Logging Management control-plane client (log groups, logs, agent
// configurations, saved searches, work requests).
func MgmtClient(inputs []*coreflow.Connection) (auth Auth, client logging.LoggingManagementClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, logging.LoggingManagementClient{}, ErrorResult(err.Error())
	}
	c, err := logging.NewLoggingManagementClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, logging.LoggingManagementClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// SearchClient is the Logging Search plane client — SearchLogs queries stored log content.
func SearchClient(inputs []*coreflow.Connection) (auth Auth, client loggingsearch.LogSearchClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, loggingsearch.LogSearchClient{}, ErrorResult(err.Error())
	}
	c, err := loggingsearch.NewLogSearchClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, loggingsearch.LogSearchClient{}, ErrorResult(a.OCIError(err))
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

func SummariseLogGroup(g *logging.LogGroup) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(g.Id),
		"display_name":       Str(g.DisplayName),
		"description":        Str(g.Description),
		"compartment_id":     Str(g.CompartmentId),
		"lifecycle_state":    string(g.LifecycleState),
		"time_created":       FormatTime(g.TimeCreated),
		"time_last_modified": FormatTime(g.TimeLastModified),
	}
}

func SummariseLogGroupSummary(g *logging.LogGroupSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(g.Id),
		"display_name":       Str(g.DisplayName),
		"description":        Str(g.Description),
		"compartment_id":     Str(g.CompartmentId),
		"lifecycle_state":    string(g.LifecycleState),
		"time_created":       FormatTime(g.TimeCreated),
		"time_last_modified": FormatTime(g.TimeLastModified),
	}
}

func SummariseLog(l *logging.Log) map[string]interface{} {
	out := map[string]interface{}{
		"id":                 Str(l.Id),
		"log_group_id":       Str(l.LogGroupId),
		"display_name":       Str(l.DisplayName),
		"log_type":           string(l.LogType),
		"is_enabled":         Bool(l.IsEnabled),
		"lifecycle_state":    string(l.LifecycleState),
		"compartment_id":     Str(l.CompartmentId),
		"retention_duration": IntOrNil(l.RetentionDuration),
		"time_created":       FormatTime(l.TimeCreated),
		"time_last_modified": FormatTime(l.TimeLastModified),
	}
	if l.Configuration != nil {
		out["configuration_compartment_id"] = Str(l.Configuration.CompartmentId)
	}
	return out
}

func SummariseLogSummary(l *logging.LogSummary) map[string]interface{} {
	out := map[string]interface{}{
		"id":                 Str(l.Id),
		"log_group_id":       Str(l.LogGroupId),
		"display_name":       Str(l.DisplayName),
		"log_type":           string(l.LogType),
		"is_enabled":         Bool(l.IsEnabled),
		"lifecycle_state":    string(l.LifecycleState),
		"compartment_id":     Str(l.CompartmentId),
		"retention_duration": IntOrNil(l.RetentionDuration),
		"time_created":       FormatTime(l.TimeCreated),
		"time_last_modified": FormatTime(l.TimeLastModified),
	}
	if l.Configuration != nil {
		out["configuration_compartment_id"] = Str(l.Configuration.CompartmentId)
	}
	return out
}

func SummariseUnifiedAgentConfiguration(c *logging.UnifiedAgentConfiguration) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(c.Id),
		"display_name":        Str(c.DisplayName),
		"description":         Str(c.Description),
		"compartment_id":      Str(c.CompartmentId),
		"is_enabled":          Bool(c.IsEnabled),
		"lifecycle_state":     string(c.LifecycleState),
		"configuration_state": string(c.ConfigurationState),
		"time_created":        FormatTime(c.TimeCreated),
		"time_last_modified":  FormatTime(c.TimeLastModified),
	}
}

func SummariseUnifiedAgentConfigurationSummary(c *logging.UnifiedAgentConfigurationSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(c.Id),
		"display_name":        Str(c.DisplayName),
		"description":         Str(c.Description),
		"compartment_id":      Str(c.CompartmentId),
		"is_enabled":          Bool(c.IsEnabled),
		"lifecycle_state":     string(c.LifecycleState),
		"configuration_type":  string(c.ConfigurationType),
		"configuration_state": string(c.ConfigurationState),
		"time_created":        FormatTime(c.TimeCreated),
		"time_last_modified":  FormatTime(c.TimeLastModified),
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

// Package functions holds what every Oracle Cloud (OCI) Functions action shares: the
// API-signing-key credential, the two clients this service spans, the resource summarisers, and
// the input/result helpers. Like the sibling OCI packages it has no Execute function, so the
// manifest generator skips it — but its category.go supplies the "Functions" sub-group.
//
// OCI Functions has two planes with DIFFERENT endpoints:
//   - FunctionsManagementClient — the REGIONAL control plane: applications and functions
//     (create/get/list/update/delete/move) plus pre-built-function listings.
//   - FunctionsInvokeClient — invocation. Each function publishes its OWN invokeEndpoint, and the
//     invoke client must be built against THAT host (mirrors the per-stream endpoint in Streaming).
//     InvokeClientForFunction resolves it automatically from the function OCID.
package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/functions"

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

// MgmtClient is the regional control-plane client (applications, functions, pbf listings).
func MgmtClient(inputs []*coreflow.Connection) (auth Auth, client functions.FunctionsManagementClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, functions.FunctionsManagementClient{}, ErrorResult(err.Error())
	}
	c, err := functions.NewFunctionsManagementClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, functions.FunctionsManagementClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// InvokeClientForFunction builds the invoke client for one function. Each function has its OWN
// invokeEndpoint, so we resolve it from the function OCID via GetFunction and build the invoke
// client against that host. The user supplies the function, never a raw endpoint.
func InvokeClientForFunction(inputs []*coreflow.Connection, functionID string) (auth Auth, client functions.FunctionsInvokeClient, errResult map[string]interface{}) {
	a, mgmt, errRes := MgmtClient(inputs)
	if errRes != nil {
		return Auth{}, functions.FunctionsInvokeClient{}, errRes
	}
	if strings.TrimSpace(functionID) == "" {
		return Auth{}, functions.FunctionsInvokeClient{}, ErrorResult("function OCID is required")
	}
	resp, err := mgmt.GetFunction(Context(), functions.GetFunctionRequest{FunctionId: &functionID})
	if err != nil {
		return Auth{}, functions.FunctionsInvokeClient{}, ErrorResult(a.OCIError(err))
	}
	endpoint := Str(resp.InvokeEndpoint)
	if endpoint == "" {
		return Auth{}, functions.FunctionsInvokeClient{}, ErrorResult(fmt.Sprintf("function %q has no invoke endpoint yet (it is %s) — wait until it is ACTIVE", Str(resp.DisplayName), string(resp.LifecycleState)))
	}
	c, err := functions.NewFunctionsInvokeClientWithConfigurationProvider(a.provider, endpoint)
	if err != nil {
		return Auth{}, functions.FunctionsInvokeClient{}, ErrorResult(a.OCIError(err))
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

func OptionalInt64(name string, inputs []*coreflow.Connection) (val int64, ok bool, err error) {
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

func ConfigMap(name string, inputs []*coreflow.Connection) (map[string]string, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object of string values, e.g. {\"KEY\":\"value\"}: %s", fieldLabel(name), err.Error())
	}
	return m, nil
}

func FreeformTags(name string, inputs []*coreflow.Connection) (map[string]string, error) {
	return ConfigMap(name, inputs)
}

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

func IntOrNil(p *int) interface{} {
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

func SummariseApplication(a *functions.Application) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(a.Id),
		"display_name":    Str(a.DisplayName),
		"compartment_id":  Str(a.CompartmentId),
		"lifecycle_state": string(a.LifecycleState),
		"config":          a.Config,
		"time_created":    FormatTime(a.TimeCreated),
	}
}

func SummariseApplicationSummary(a *functions.ApplicationSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(a.Id),
		"display_name":    Str(a.DisplayName),
		"compartment_id":  Str(a.CompartmentId),
		"lifecycle_state": string(a.LifecycleState),
		"time_created":    FormatTime(a.TimeCreated),
	}
}

func SummariseFunction(f *functions.Function) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(f.Id),
		"display_name":       Str(f.DisplayName),
		"application_id":     Str(f.ApplicationId),
		"compartment_id":     Str(f.CompartmentId),
		"lifecycle_state":    string(f.LifecycleState),
		"image":              Str(f.Image),
		"memory_in_mbs":      Int64OrNil(f.MemoryInMBs),
		"timeout_in_seconds": IntOrNil(f.TimeoutInSeconds),
		"invoke_endpoint":    Str(f.InvokeEndpoint),
		"config":             f.Config,
		"time_created":       FormatTime(f.TimeCreated),
	}
}

func SummariseFunctionSummary(f *functions.FunctionSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(f.Id),
		"display_name":    Str(f.DisplayName),
		"application_id":  Str(f.ApplicationId),
		"compartment_id":  Str(f.CompartmentId),
		"lifecycle_state": string(f.LifecycleState),
		"image":           Str(f.Image),
		"invoke_endpoint": Str(f.InvokeEndpoint),
		"time_created":    FormatTime(f.TimeCreated),
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

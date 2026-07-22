// Package apigateway holds what every Oracle Cloud (OCI) API Gateway action shares: the
// API-signing-key credential, the three clients this service spans, the resource summarisers, and
// the input/result helpers. Like the sibling OCI packages it has no Execute function, so the
// manifest generator skips it — but its category.go supplies the "API Gateway" sub-group.
//
// OCI API Gateway is a single regional service split across three control-plane clients that share
// one signing-key provider:
//   - GatewayClient — the virtual network appliances (gateways) that route inbound traffic.
//   - DeploymentClient — deployments that publish an API on a gateway under a path prefix.
//   - ApiGatewayClient — the API resources: containers for the versioned API specifications served.
package apigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
	"github.com/oracle/oci-go-sdk/v65/common"

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

// GatewayClient is the regional client for gateways (the virtual network appliances).
func GatewayClient(inputs []*coreflow.Connection) (auth Auth, client apigateway.GatewayClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, apigateway.GatewayClient{}, ErrorResult(err.Error())
	}
	c, err := apigateway.NewGatewayClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, apigateway.GatewayClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// DeploymentClient is the regional client for deployments (an API published on a gateway).
func DeploymentClient(inputs []*coreflow.Connection) (auth Auth, client apigateway.DeploymentClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, apigateway.DeploymentClient{}, ErrorResult(err.Error())
	}
	c, err := apigateway.NewDeploymentClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, apigateway.DeploymentClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// ApiClient is the regional client for API resources (containers for API specifications).
func ApiClient(inputs []*coreflow.Connection) (auth Auth, client apigateway.ApiGatewayClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, apigateway.ApiGatewayClient{}, ErrorResult(err.Error())
	}
	c, err := apigateway.NewApiGatewayClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, apigateway.ApiGatewayClient{}, ErrorResult(a.OCIError(err))
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

func SummariseGateway(r *apigateway.Gateway) map[string]interface{} {
	return map[string]interface{}{
		"id":                         Str(r.Id),
		"display_name":               Str(r.DisplayName),
		"compartment_id":             Str(r.CompartmentId),
		"endpoint_type":              string(r.EndpointType),
		"subnet_id":                  Str(r.SubnetId),
		"hostname":                   Str(r.Hostname),
		"certificate_id":             Str(r.CertificateId),
		"ip_mode":                    string(r.IpMode),
		"network_security_group_ids": r.NetworkSecurityGroupIds,
		"lifecycle_state":            string(r.LifecycleState),
		"lifecycle_details":          Str(r.LifecycleDetails),
		"time_created":               FormatTime(r.TimeCreated),
		"time_updated":               FormatTime(r.TimeUpdated),
	}
}

func SummariseGatewaySummary(r *apigateway.GatewaySummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(r.Id),
		"display_name":    Str(r.DisplayName),
		"compartment_id":  Str(r.CompartmentId),
		"endpoint_type":   string(r.EndpointType),
		"subnet_id":       Str(r.SubnetId),
		"hostname":        Str(r.Hostname),
		"certificate_id":  Str(r.CertificateId),
		"ip_mode":         string(r.IpMode),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
	}
}

func SummariseDeployment(r *apigateway.Deployment) map[string]interface{} {
	return map[string]interface{}{
		"id":                Str(r.Id),
		"display_name":      Str(r.DisplayName),
		"compartment_id":    Str(r.CompartmentId),
		"gateway_id":        Str(r.GatewayId),
		"path_prefix":       Str(r.PathPrefix),
		"endpoint":          Str(r.Endpoint),
		"lifecycle_state":   string(r.LifecycleState),
		"lifecycle_details": Str(r.LifecycleDetails),
		"time_created":      FormatTime(r.TimeCreated),
		"time_updated":      FormatTime(r.TimeUpdated),
	}
}

func SummariseDeploymentSummary(r *apigateway.DeploymentSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(r.Id),
		"display_name":    Str(r.DisplayName),
		"compartment_id":  Str(r.CompartmentId),
		"gateway_id":      Str(r.GatewayId),
		"path_prefix":     Str(r.PathPrefix),
		"endpoint":        Str(r.Endpoint),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
	}
}

func SummariseApi(r *apigateway.Api) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(r.Id),
		"display_name":       Str(r.DisplayName),
		"compartment_id":     Str(r.CompartmentId),
		"specification_type": Str(r.SpecificationType),
		"lifecycle_state":    string(r.LifecycleState),
		"lifecycle_details":  Str(r.LifecycleDetails),
		"time_created":       FormatTime(r.TimeCreated),
		"time_updated":       FormatTime(r.TimeUpdated),
	}
}

func SummariseApiSummary(r *apigateway.ApiSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(r.Id),
		"display_name":       Str(r.DisplayName),
		"compartment_id":     Str(r.CompartmentId),
		"specification_type": Str(r.SpecificationType),
		"lifecycle_state":    string(r.LifecycleState),
		"lifecycle_details":  Str(r.LifecycleDetails),
		"time_created":       FormatTime(r.TimeCreated),
		"time_updated":       FormatTime(r.TimeUpdated),
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

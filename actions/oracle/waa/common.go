// Package waa holds what every Oracle Cloud (OCI) Web Application Acceleration action shares: the
// API-signing-key credential, the regional WaaClient, the WebAppAcceleration / policy summarisers,
// and the input/result helpers. Like the sibling OCI packages it has no Execute function, so the
// manifest generator skips it — but its category.go supplies the "Web Application Acceleration"
// sub-group.
//
// OCI Web App Acceleration is a regional service: an acceleration policy bundles caching and
// compression rules, a WebAppAcceleration attaches that policy to a load-balancer backend, and the
// cache can be purged on demand.
package waa

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/waa"

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

// Client is the regional Web App Acceleration client.
func Client(inputs []*coreflow.Connection) (auth Auth, client waa.WaaClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, waa.WaaClient{}, ErrorResult(err.Error())
	}
	c, err := waa.NewWaaClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, waa.WaaClient{}, ErrorResult(a.OCIError(err))
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

func FormatTime(t *common.SDKTime) string {
	if t == nil {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// Resource summarisers
// ---------------------------------------------------------------------------

// SummariseWebAppAcceleration flattens the polymorphic WebAppAcceleration interface. The only
// backend type today is LOAD_BALANCER, whose concrete value carries the load-balancer OCID.
func SummariseWebAppAcceleration(r waa.WebAppAcceleration) map[string]interface{} {
	if r == nil {
		return nil
	}
	out := map[string]interface{}{
		"id":                             Str(r.GetId()),
		"display_name":                   Str(r.GetDisplayName()),
		"compartment_id":                 Str(r.GetCompartmentId()),
		"web_app_acceleration_policy_id": Str(r.GetWebAppAccelerationPolicyId()),
		"lifecycle_state":                string(r.GetLifecycleState()),
		"lifecycle_details":              Str(r.GetLifecycleDetails()),
		"time_created":                   FormatTime(r.GetTimeCreated()),
		"time_updated":                   FormatTime(r.GetTimeUpdated()),
	}
	if lb, ok := r.(waa.WebAppAccelerationLoadBalancer); ok {
		out["backend_type"] = "LOAD_BALANCER"
		out["load_balancer_id"] = Str(lb.LoadBalancerId)
	}
	return out
}

// SummariseWebAppAccelerationSummary flattens the polymorphic list-item interface.
func SummariseWebAppAccelerationSummary(r waa.WebAppAccelerationSummary) map[string]interface{} {
	if r == nil {
		return nil
	}
	out := map[string]interface{}{
		"id":                             Str(r.GetId()),
		"display_name":                   Str(r.GetDisplayName()),
		"compartment_id":                 Str(r.GetCompartmentId()),
		"web_app_acceleration_policy_id": Str(r.GetWebAppAccelerationPolicyId()),
		"lifecycle_state":                string(r.GetLifecycleState()),
		"lifecycle_details":              Str(r.GetLifecycleDetails()),
		"time_created":                   FormatTime(r.GetTimeCreated()),
		"time_updated":                   FormatTime(r.GetTimeUpdated()),
	}
	if lb, ok := r.(waa.WebAppAccelerationLoadBalancerSummary); ok {
		out["backend_type"] = "LOAD_BALANCER"
		out["load_balancer_id"] = Str(lb.LoadBalancerId)
	}
	return out
}

func SummariseWebAppAccelerationPolicy(r *waa.WebAppAccelerationPolicy) map[string]interface{} {
	if r == nil {
		return nil
	}
	return map[string]interface{}{
		"id":                     Str(r.Id),
		"display_name":           Str(r.DisplayName),
		"compartment_id":         Str(r.CompartmentId),
		"lifecycle_state":        string(r.LifecycleState),
		"lifecycle_details":      Str(r.LifecycleDetails),
		"has_caching_policy":     r.ResponseCachingPolicy != nil,
		"has_compression_policy": r.ResponseCompressionPolicy != nil,
		"time_created":           FormatTime(r.TimeCreated),
		"time_updated":           FormatTime(r.TimeUpdated),
	}
}

func SummariseWebAppAccelerationPolicySummary(r *waa.WebAppAccelerationPolicySummary) map[string]interface{} {
	if r == nil {
		return nil
	}
	return map[string]interface{}{
		"id":                Str(r.Id),
		"display_name":      Str(r.DisplayName),
		"compartment_id":    Str(r.CompartmentId),
		"lifecycle_state":   string(r.LifecycleState),
		"lifecycle_details": Str(r.LifecycleDetails),
		"time_created":      FormatTime(r.TimeCreated),
		"time_updated":      FormatTime(r.TimeUpdated),
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

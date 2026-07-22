// Package waf holds what every Oracle Cloud (OCI) Web Application Firewall action shares: the
// API-signing-key credential, the regional WafClient, the WebAppFirewall / policy / network
// address list summarisers, and the input/result helpers. Like the sibling OCI packages it has no
// Execute function, so the manifest generator skips it — but its category.go supplies the
// "Web Application Firewall" sub-group.
//
// OCI WAF is a single regional service: a WebAppFirewall attaches a reusable WebAppFirewallPolicy
// to a backend (currently a load balancer), and NetworkAddressLists are IP/VCN address sets shared
// across policies. Several resources are polymorphic (WebAppFirewall by backend type,
// NetworkAddressList by address type), so their summarisers take the SDK interface and read the
// common GetXxx() accessors.
package waf

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/waf"

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

// Client is the regional Web Application Firewall client.
func Client(inputs []*coreflow.Connection) (auth Auth, client waf.WafClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, waf.WafClient{}, ErrorResult(err.Error())
	}
	c, err := waf.NewWafClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, waf.WafClient{}, ErrorResult(a.OCIError(err))
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

// IntOrNil returns a *int for the field, or nil when blank so a partial update leaves it unchanged.
func IntOrNil(name string, inputs []*coreflow.Connection) (*int, error) {
	n, ok, err := OptionalInt(name, inputs)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return &n, nil
}

// Int64OrNil returns a *int64 for the field, or nil when blank.
func Int64OrNil(name string, inputs []*coreflow.Connection) (*int64, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be a whole number", fieldLabel(name))
	}
	return &n, nil
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

func IntOrNilVal(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func Int64OrNilVal(p *int64) interface{} {
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

// SummariseWebAppFirewall reads the common accessors of the polymorphic WebAppFirewall interface,
// adding the load-balancer-specific fields when the backend is a LOAD_BALANCER.
func SummariseWebAppFirewall(f waf.WebAppFirewall) map[string]interface{} {
	if f == nil {
		return nil
	}
	out := map[string]interface{}{
		"id":                         Str(f.GetId()),
		"display_name":               Str(f.GetDisplayName()),
		"compartment_id":             Str(f.GetCompartmentId()),
		"web_app_firewall_policy_id": Str(f.GetWebAppFirewallPolicyId()),
		"lifecycle_state":            string(f.GetLifecycleState()),
		"lifecycle_details":          Str(f.GetLifecycleDetails()),
		"time_created":               FormatTime(f.GetTimeCreated()),
		"time_updated":               FormatTime(f.GetTimeUpdated()),
	}
	if lb, ok := f.(waf.WebAppFirewallLoadBalancer); ok {
		out["backend_type"] = "LOAD_BALANCER"
		out["load_balancer_id"] = Str(lb.LoadBalancerId)
	}
	return out
}

// SummariseWebAppFirewallSummary reads the common accessors of the polymorphic
// WebAppFirewallSummary interface, adding the load-balancer id when present.
func SummariseWebAppFirewallSummary(f waf.WebAppFirewallSummary) map[string]interface{} {
	if f == nil {
		return nil
	}
	out := map[string]interface{}{
		"id":                         Str(f.GetId()),
		"display_name":               Str(f.GetDisplayName()),
		"compartment_id":             Str(f.GetCompartmentId()),
		"web_app_firewall_policy_id": Str(f.GetWebAppFirewallPolicyId()),
		"lifecycle_state":            string(f.GetLifecycleState()),
		"lifecycle_details":          Str(f.GetLifecycleDetails()),
		"time_created":               FormatTime(f.GetTimeCreated()),
		"time_updated":               FormatTime(f.GetTimeUpdated()),
	}
	if lb, ok := f.(waf.WebAppFirewallLoadBalancerSummary); ok {
		out["backend_type"] = "LOAD_BALANCER"
		out["load_balancer_id"] = Str(lb.LoadBalancerId)
	}
	return out
}

func SummariseWebAppFirewallPolicy(p *waf.WebAppFirewallPolicy) map[string]interface{} {
	if p == nil {
		return nil
	}
	return map[string]interface{}{
		"id":                Str(p.Id),
		"display_name":      Str(p.DisplayName),
		"compartment_id":    Str(p.CompartmentId),
		"lifecycle_state":   string(p.LifecycleState),
		"lifecycle_details": Str(p.LifecycleDetails),
		"actions_count":     len(p.Actions),
		"time_created":      FormatTime(p.TimeCreated),
		"time_updated":      FormatTime(p.TimeUpdated),
	}
}

func SummariseWebAppFirewallPolicySummary(p *waf.WebAppFirewallPolicySummary) map[string]interface{} {
	if p == nil {
		return nil
	}
	return map[string]interface{}{
		"id":                Str(p.Id),
		"display_name":      Str(p.DisplayName),
		"compartment_id":    Str(p.CompartmentId),
		"lifecycle_state":   string(p.LifecycleState),
		"lifecycle_details": Str(p.LifecycleDetails),
		"time_created":      FormatTime(p.TimeCreated),
		"time_updated":      FormatTime(p.TimeUpdated),
	}
}

// SummariseNetworkAddressList reads the common accessors of the polymorphic NetworkAddressList
// interface, adding the address type and count from the concrete ADDRESSES / VCN_ADDRESSES impls.
func SummariseNetworkAddressList(l waf.NetworkAddressList) map[string]interface{} {
	if l == nil {
		return nil
	}
	out := map[string]interface{}{
		"id":                Str(l.GetId()),
		"display_name":      Str(l.GetDisplayName()),
		"compartment_id":    Str(l.GetCompartmentId()),
		"lifecycle_state":   string(l.GetLifecycleState()),
		"lifecycle_details": Str(l.GetLifecycleDetails()),
		"time_created":      FormatTime(l.GetTimeCreated()),
		"time_updated":      FormatTime(l.GetTimeUpdated()),
	}
	switch v := l.(type) {
	case waf.NetworkAddressListAddresses:
		out["type"] = "ADDRESSES"
		out["address_count"] = len(v.Addresses)
	case waf.NetworkAddressListVcnAddresses:
		out["type"] = "VCN_ADDRESSES"
		out["address_count"] = len(v.VcnAddresses)
	}
	return out
}

// SummariseNetworkAddressListSummary reads the common accessors of the polymorphic
// NetworkAddressListSummary interface, adding the address type and count from the concrete impls.
func SummariseNetworkAddressListSummary(l waf.NetworkAddressListSummary) map[string]interface{} {
	if l == nil {
		return nil
	}
	out := map[string]interface{}{
		"id":                Str(l.GetId()),
		"display_name":      Str(l.GetDisplayName()),
		"compartment_id":    Str(l.GetCompartmentId()),
		"lifecycle_state":   string(l.GetLifecycleState()),
		"lifecycle_details": Str(l.GetLifecycleDetails()),
		"time_created":      FormatTime(l.GetTimeCreated()),
		"time_updated":      FormatTime(l.GetTimeUpdated()),
	}
	switch v := l.(type) {
	case waf.NetworkAddressListAddressesSummary:
		out["type"] = "ADDRESSES"
		out["address_count"] = len(v.Addresses)
	case waf.NetworkAddressListVcnAddressesSummary:
		out["type"] = "VCN_ADDRESSES"
		out["address_count"] = len(v.VcnAddresses)
	}
	return out
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

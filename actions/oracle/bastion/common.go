// Package bastion holds what every Oracle Cloud (OCI) Bastion action shares: the API-signing-key
// credential, the regional BastionClient, the Bastion/Session summarisers, and the input/result
// helpers. Like the sibling OCI packages it has no Execute function, so the manifest generator
// skips it — but its category.go supplies the "Bastion" sub-group.
//
// OCI Bastion provides restricted, time-limited access to target resources that have no public
// endpoint: a bastion resides in a public subnet and brokers Secure Shell (SSH) sessions —
// managed SSH, port forwarding, or dynamic port forwarding — from allow-listed client IPs to
// hosts in a private subnet.
package bastion

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/bastion"
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

// Client is the regional Bastion client.
func Client(inputs []*coreflow.Connection) (auth Auth, client bastion.BastionClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, bastion.BastionClient{}, ErrorResult(err.Error())
	}
	c, err := bastion.NewBastionClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, bastion.BastionClient{}, ErrorResult(a.OCIError(err))
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

// OptionalInt returns nil when the field is blank so a partial update leaves it unchanged.
func OptionalInt(name string, inputs []*coreflow.Connection) (*int, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var v int
	if _, err := fmt.Sscanf(raw, "%d", &v); err != nil {
		return nil, fmt.Errorf("%s must be a whole number", fieldLabel(name))
	}
	return &v, nil
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

func SummariseBastion(r *bastion.Bastion) map[string]interface{} {
	return map[string]interface{}{
		"id":                           Str(r.Id),
		"name":                         Str(r.Name),
		"bastion_type":                 Str(r.BastionType),
		"compartment_id":               Str(r.CompartmentId),
		"target_vcn_id":                Str(r.TargetVcnId),
		"target_subnet_id":             Str(r.TargetSubnetId),
		"max_session_ttl_in_seconds":   IntOrNil(r.MaxSessionTtlInSeconds),
		"max_sessions_allowed":         IntOrNil(r.MaxSessionsAllowed),
		"private_endpoint_ip_address":  Str(r.PrivateEndpointIpAddress),
		"client_cidr_block_allow_list": r.ClientCidrBlockAllowList,
		"dns_proxy_status":             string(r.DnsProxyStatus),
		"lifecycle_state":              string(r.LifecycleState),
		"lifecycle_details":            Str(r.LifecycleDetails),
		"time_created":                 FormatTime(r.TimeCreated),
		"time_updated":                 FormatTime(r.TimeUpdated),
	}
}

func SummariseBastionSummary(r *bastion.BastionSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                Str(r.Id),
		"name":              Str(r.Name),
		"bastion_type":      Str(r.BastionType),
		"compartment_id":    Str(r.CompartmentId),
		"target_vcn_id":     Str(r.TargetVcnId),
		"target_subnet_id":  Str(r.TargetSubnetId),
		"dns_proxy_status":  string(r.DnsProxyStatus),
		"lifecycle_state":   string(r.LifecycleState),
		"lifecycle_details": Str(r.LifecycleDetails),
		"time_created":      FormatTime(r.TimeCreated),
		"time_updated":      FormatTime(r.TimeUpdated),
	}
}

func SummariseSession(r *bastion.Session) map[string]interface{} {
	out := map[string]interface{}{
		"id":                     Str(r.Id),
		"display_name":           Str(r.DisplayName),
		"bastion_id":             Str(r.BastionId),
		"bastion_name":           Str(r.BastionName),
		"bastion_user_name":      Str(r.BastionUserName),
		"session_ttl_in_seconds": IntOrNil(r.SessionTtlInSeconds),
		"key_type":               string(r.KeyType),
		"lifecycle_state":        string(r.LifecycleState),
		"lifecycle_details":      Str(r.LifecycleDetails),
		"time_created":           FormatTime(r.TimeCreated),
		"time_updated":           FormatTime(r.TimeUpdated),
	}
	for k, v := range summariseTargetResource(r.TargetResourceDetails) {
		out[k] = v
	}
	return out
}

func SummariseSessionSummary(r *bastion.SessionSummary) map[string]interface{} {
	out := map[string]interface{}{
		"id":                     Str(r.Id),
		"display_name":           Str(r.DisplayName),
		"bastion_id":             Str(r.BastionId),
		"bastion_name":           Str(r.BastionName),
		"session_ttl_in_seconds": IntOrNil(r.SessionTtlInSeconds),
		"lifecycle_state":        string(r.LifecycleState),
		"lifecycle_details":      Str(r.LifecycleDetails),
		"time_created":           FormatTime(r.TimeCreated),
		"time_updated":           FormatTime(r.TimeUpdated),
	}
	for k, v := range summariseTargetResource(r.TargetResourceDetails) {
		out[k] = v
	}
	return out
}

// summariseTargetResource flattens the polymorphic TargetResourceDetails (a session is one of
// MANAGED_SSH, PORT_FORWARDING or DYNAMIC_PORT_FORWARDING) into a common set of keys.
func summariseTargetResource(t bastion.TargetResourceDetails) map[string]interface{} {
	out := map[string]interface{}{}
	switch d := t.(type) {
	case bastion.ManagedSshSessionTargetResourceDetails:
		out["session_type"] = "MANAGED_SSH"
		out["target_resource_id"] = Str(d.TargetResourceId)
		out["target_resource_display_name"] = Str(d.TargetResourceDisplayName)
		out["target_resource_private_ip_address"] = Str(d.TargetResourcePrivateIpAddress)
		out["target_resource_port"] = IntOrNil(d.TargetResourcePort)
		out["target_resource_operating_system_user_name"] = Str(d.TargetResourceOperatingSystemUserName)
	case bastion.PortForwardingSessionTargetResourceDetails:
		out["session_type"] = "PORT_FORWARDING"
		out["target_resource_id"] = Str(d.TargetResourceId)
		out["target_resource_display_name"] = Str(d.TargetResourceDisplayName)
		out["target_resource_private_ip_address"] = Str(d.TargetResourcePrivateIpAddress)
		out["target_resource_fqdn"] = Str(d.TargetResourceFqdn)
		out["target_resource_port"] = IntOrNil(d.TargetResourcePort)
	case bastion.DynamicPortForwardingSessionTargetResourceDetails:
		out["session_type"] = "DYNAMIC_PORT_FORWARDING"
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

// Package networkloadbalancer holds what every Oracle Cloud (OCI) Network Load
// Balancer action shares: the API-signing-key credential, the NetworkLoadBalancer
// client factory, the two per-scope client preambles, the resource summarisers, and
// the input/result helpers. Like the sibling OCI packages it has no Execute function,
// so the manifest generator skips it — but its category.go supplies the sub-group.
//
// Like the classic Load Balancer, the NLB API is ASYNCHRONOUS: mutating calls return
// an opc-work-request-id, surfaced directly (fire-and-return) via AsyncResult with
// work_request_get/list to poll. Two differences from the classic LB: CreateNetwork-
// LoadBalancer returns the resource body AS WELL AS a work-request id (so create can
// hand back the OCID immediately), and the details structs use TYPED enums (Policy,
// Protocol) rather than *string — hence ValidateEnum returns the canonical value that
// callers convert to the SDK enum type.
package networkloadbalancer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"

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

// Allowed enum sets, confirmed against the live OCI Network Load Balancer API.
var (
	NlbPolicies          = []string{"TWO_TUPLE", "THREE_TUPLE", "FIVE_TUPLE"}
	ListenerProtocols    = []string{"ANY", "TCP", "UDP", "TCP_AND_UDP", "L3IP"}
	HealthCheckProtocols = []string{"HTTP", "HTTPS", "TCP", "UDP", "DNS"}
)

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

// GetAuth reads the standard credential block and builds the signing-key provider,
// validating the host-selecting region and eagerly parsing the PEM.
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

// NetworkLoadBalancerClient builds the single service client the whole node uses.
func (a Auth) NetworkLoadBalancerClient() (nlb.NetworkLoadBalancerClient, error) {
	return nlb.NewNetworkLoadBalancerClientWithConfigurationProvider(a.provider)
}

// Client is the preamble for compartment-scoped ops (create, list).
func Client(inputs []*coreflow.Connection) (auth Auth, client nlb.NetworkLoadBalancerClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, nlb.NetworkLoadBalancerClient{}, ErrorResult(err.Error())
	}
	c, err := a.NetworkLoadBalancerClient()
	if err != nil {
		return Auth{}, nlb.NetworkLoadBalancerClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// ResourceClient is the preamble for per-NLB ops: it additionally reads one resource
// OCID (named by ocidInputName, usually "network_load_balancer_ocid").
func ResourceClient(inputs []*coreflow.Connection, ocidInputName string) (auth Auth, client nlb.NetworkLoadBalancerClient, ocid string, errResult map[string]interface{}) {
	a, c, errRes := Client(inputs)
	if errRes != nil {
		return Auth{}, nlb.NetworkLoadBalancerClient{}, "", errRes
	}
	id, err := RequiredString(ocidInputName, inputs)
	if err != nil {
		return Auth{}, nlb.NetworkLoadBalancerClient{}, "", ErrorResult(err.Error())
	}
	return a, c, id, nil
}

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

// ValidateEnum upper-cases an operator-entered enum value and checks it against the
// allowed set, returning the canonical (upper-case) value or a helpful error. The
// NLB details structs use typed enums, so callers convert the returned string to the
// SDK enum type. Accepts lower-case input and catches typos up front.
func ValidateEnum(field, value string, allowed ...string) (string, error) {
	v := strings.ToUpper(strings.TrimSpace(value))
	for _, a := range allowed {
		if v == a {
			return v, nil
		}
	}
	return "", fmt.Errorf("%s must be one of: %s", field, strings.Join(allowed, ", "))
}

func Str(p *string) string {
	if p == nil {
		return ""
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

func SummariseNetworkLoadBalancer(n *nlb.NetworkLoadBalancer) map[string]interface{} {
	ips := make([]string, 0, len(n.IpAddresses))
	for _, ip := range n.IpAddresses {
		ips = append(ips, Str(ip.IpAddress))
	}
	backendSets := make([]string, 0, len(n.BackendSets))
	for name := range n.BackendSets {
		backendSets = append(backendSets, name)
	}
	listeners := make([]string, 0, len(n.Listeners))
	for name := range n.Listeners {
		listeners = append(listeners, name)
	}
	return map[string]interface{}{
		"id":                         Str(n.Id),
		"display_name":               Str(n.DisplayName),
		"compartment_id":             Str(n.CompartmentId),
		"lifecycle_state":            string(n.LifecycleState),
		"is_private":                 n.IsPrivate != nil && *n.IsPrivate,
		"subnet_id":                  Str(n.SubnetId),
		"nlb_ip_version":             string(n.NlbIpVersion),
		"ip_addresses":               ips,
		"network_security_group_ids": n.NetworkSecurityGroupIds,
		"backend_set_names":          backendSets,
		"listener_names":             listeners,
		"time_created":               FormatTime(n.TimeCreated),
	}
}

// SummariseNetworkLoadBalancerSummary mirrors SummariseNetworkLoadBalancer for the
// list endpoint, which returns the distinct (but field-identical) summary type.
func SummariseNetworkLoadBalancerSummary(n *nlb.NetworkLoadBalancerSummary) map[string]interface{} {
	ips := make([]string, 0, len(n.IpAddresses))
	for _, ip := range n.IpAddresses {
		ips = append(ips, Str(ip.IpAddress))
	}
	backendSets := make([]string, 0, len(n.BackendSets))
	for name := range n.BackendSets {
		backendSets = append(backendSets, name)
	}
	listeners := make([]string, 0, len(n.Listeners))
	for name := range n.Listeners {
		listeners = append(listeners, name)
	}
	return map[string]interface{}{
		"id":                         Str(n.Id),
		"display_name":               Str(n.DisplayName),
		"compartment_id":             Str(n.CompartmentId),
		"lifecycle_state":            string(n.LifecycleState),
		"is_private":                 n.IsPrivate != nil && *n.IsPrivate,
		"subnet_id":                  Str(n.SubnetId),
		"nlb_ip_version":             string(n.NlbIpVersion),
		"ip_addresses":               ips,
		"network_security_group_ids": n.NetworkSecurityGroupIds,
		"backend_set_names":          backendSets,
		"listener_names":             listeners,
		"time_created":               FormatTime(n.TimeCreated),
	}
}

func SummariseBackendSet(bs *nlb.BackendSet) map[string]interface{} {
	backends := make([]map[string]interface{}, 0, len(bs.Backends))
	for i := range bs.Backends {
		backends = append(backends, SummariseBackend(&bs.Backends[i]))
	}
	m := map[string]interface{}{
		"name":               Str(bs.Name),
		"policy":             string(bs.Policy),
		"is_preserve_source": bs.IsPreserveSource != nil && *bs.IsPreserveSource,
		"backends":           backends,
	}
	if bs.HealthChecker != nil {
		m["health_checker"] = SummariseHealthChecker(bs.HealthChecker)
	}
	return m
}

func SummariseBackend(b *nlb.Backend) map[string]interface{} {
	m := map[string]interface{}{
		"name":       Str(b.Name),
		"ip_address": Str(b.IpAddress),
		"target_id":  Str(b.TargetId),
		"is_drain":   b.IsDrain != nil && *b.IsDrain,
		"is_backup":  b.IsBackup != nil && *b.IsBackup,
		"is_offline": b.IsOffline != nil && *b.IsOffline,
	}
	if b.Port != nil {
		m["port"] = *b.Port
	}
	if b.Weight != nil {
		m["weight"] = *b.Weight
	}
	return m
}

func SummariseListener(l *nlb.Listener) map[string]interface{} {
	m := map[string]interface{}{
		"name":                     Str(l.Name),
		"default_backend_set_name": Str(l.DefaultBackendSetName),
		"protocol":                 string(l.Protocol),
		"ip_version":               string(l.IpVersion),
	}
	if l.Port != nil {
		m["port"] = *l.Port
	}
	return m
}

func SummariseHealthChecker(h *nlb.HealthChecker) map[string]interface{} {
	m := map[string]interface{}{
		"protocol": string(h.Protocol),
		"url_path": Str(h.UrlPath),
	}
	if h.Port != nil {
		m["port"] = *h.Port
	}
	if h.ReturnCode != nil {
		m["return_code"] = *h.ReturnCode
	}
	if h.Retries != nil {
		m["retries"] = *h.Retries
	}
	if h.IntervalInMillis != nil {
		m["interval_in_millis"] = *h.IntervalInMillis
	}
	if h.TimeoutInMillis != nil {
		m["timeout_in_millis"] = *h.TimeoutInMillis
	}
	return m
}

func SummariseWorkRequest(w *nlb.WorkRequest) map[string]interface{} {
	return map[string]interface{}{
		"id":               Str(w.Id),
		"operation_type":   string(w.OperationType),
		"status":           string(w.Status),
		"compartment_id":   Str(w.CompartmentId),
		"percent_complete": w.PercentComplete,
		"time_accepted":    FormatTime(w.TimeAccepted),
		"time_finished":    FormatTime(w.TimeFinished),
	}
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

// AsyncResult is the standard envelope for the work-request-driven operations.
func AsyncResult(msg, workRequestID string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result":     msg,
		"work_request_id": workRequestID,
		"success":         true,
	}
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

// Context is what every action passes: no request deadline from the executor.
func Context() context.Context { return context.Background() }

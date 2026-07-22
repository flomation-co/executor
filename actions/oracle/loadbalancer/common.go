// Package loadbalancer holds what every Oracle Cloud (OCI) Load Balancer action
// shares: the API-signing-key credential, the single LoadBalancerClient factory, the
// two per-scope client preambles, the resource summarisers, and the input/result
// helpers. Like the sibling OCI packages it has no Execute function, so the manifest
// generator skips it — but its category.go supplies the "Load Balancer" sub-group.
//
// The classic OCI Load Balancer API is heavily ASYNCHRONOUS: nearly every mutating
// call (create/update/delete + change-compartment + shape/NSG updates) returns only
// an opc-work-request-id and NO resource body. We surface that id directly
// (fire-and-return) via AsyncResult, and expose work_request_get/list so a flow can
// poll it — the same non-blocking model the Autonomous Database and Networking nodes
// use, so a flow never hangs for a minutes-long provision. Reads (get/list/health)
// are synchronous and summarise the resource inline.
//
// As with the siblings, the manifest generator only resolves INLINE Inputs literals,
// so the credential + compartment input declarations must be copy-pasted into each
// action's Inputs.
package loadbalancer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"

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

// ListMaxPages bounds every list action's pagination walk so one node run can't turn
// into an unbounded sequence of API calls; a walk that hits the cap sets truncated.
const ListMaxPages = 25

var validRegion = regexp.MustCompile(`^[a-z0-9-]+$`)

// Auth carries the API-signing-key material plus the compartment scope. The parsed
// ConfigurationProvider owns the request signer and backs the LoadBalancer client.
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
// ConfigurationProvider, validating the host-selecting region and eagerly parsing the
// PEM so a bad key fails cleanly up front.
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

// LoadBalancerClient builds the single service client the whole node uses.
func (a Auth) LoadBalancerClient() (loadbalancer.LoadBalancerClient, error) {
	return loadbalancer.NewLoadBalancerClientWithConfigurationProvider(a.provider)
}

// Client is the preamble for compartment-scoped ops (create, list): it reads the
// credential block and builds the client. On any error it returns a ready ErrorResult.
func Client(inputs []*coreflow.Connection) (auth Auth, client loadbalancer.LoadBalancerClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, loadbalancer.LoadBalancerClient{}, ErrorResult(err.Error())
	}
	c, err := a.LoadBalancerClient()
	if err != nil {
		return Auth{}, loadbalancer.LoadBalancerClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// ResourceClient is the preamble for per-load-balancer ops: it additionally reads one
// resource OCID (named by ocidInputName, usually "load_balancer_ocid").
func ResourceClient(inputs []*coreflow.Connection, ocidInputName string) (auth Auth, client loadbalancer.LoadBalancerClient, ocid string, errResult map[string]interface{}) {
	a, c, errRes := Client(inputs)
	if errRes != nil {
		return Auth{}, loadbalancer.LoadBalancerClient{}, "", errRes
	}
	id, err := RequiredString(ocidInputName, inputs)
	if err != nil {
		return Auth{}, loadbalancer.LoadBalancerClient{}, "", ErrorResult(err.Error())
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

// BoolWasSet reports whether the operator provided any value for a boolean input.
func BoolWasSet(name string, inputs []*coreflow.Connection) bool {
	return strings.TrimSpace(OptionalString(name, inputs)) != ""
}

// OptionalInt reads a whole-number input into an *int (ports, weights, retries),
// returning (value, true) when set, (0, false) when blank, or an error when present
// but not an integer.
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

// RequiredInt reads a mandatory whole-number input.
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

// FreeformTags parses a JSON object input into map[string]string; blank → nil.
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

func FormatTime(t *common.SDKTime) string {
	if t == nil {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// ValidateEnum upper-cases an operator-entered constrained-string value and checks it
// against the allowed set, returning the canonical (upper-case) value or a helpful
// error listing the choices. OCI enum values are upper-case, so this accepts
// lower-case input (a common slip, e.g. "round_robin") and catches typos up front
// rather than letting a raw OCI rejection surface mid-flow. The live pickers supply
// the correct value in the editor; this guards programmatic/chained callers.
func ValidateEnum(field, value string, allowed ...string) (string, error) {
	v := strings.ToUpper(strings.TrimSpace(value))
	for _, a := range allowed {
		if v == a {
			return v, nil
		}
	}
	return "", fmt.Errorf("%s must be one of: %s", field, strings.Join(allowed, ", "))
}

// Allowed enum sets, confirmed against the live OCI Load Balancer API.
var (
	BackendPolicies      = []string{"ROUND_ROBIN", "LEAST_CONNECTIONS", "IP_HASH"}
	ListenerProtocols    = []string{"HTTP", "HTTP2", "HTTP3", "TCP", "GRPC"}
	HealthCheckProtocols = []string{"HTTP", "HTTPS", "TCP"}
)

// NormaliseRegion trims and lower-cases an operator-entered OCI region identifier —
// region keys are canonically lower-case, so we normalise free-text input the same
// way GetAuth treats the auth region.
func NormaliseRegion(name string, inputs []*coreflow.Connection) string {
	return strings.ToLower(strings.TrimSpace(OptionalString(name, inputs)))
}

// ---------------------------------------------------------------------------
// Resource summarisers
// ---------------------------------------------------------------------------

func SummariseLoadBalancer(lb *loadbalancer.LoadBalancer) map[string]interface{} {
	ips := make([]string, 0, len(lb.IpAddresses))
	for _, ip := range lb.IpAddresses {
		ips = append(ips, Str(ip.IpAddress))
	}
	backendSets := make([]string, 0, len(lb.BackendSets))
	for name := range lb.BackendSets {
		backendSets = append(backendSets, name)
	}
	listeners := make([]string, 0, len(lb.Listeners))
	for name := range lb.Listeners {
		listeners = append(listeners, name)
	}
	return map[string]interface{}{
		"id":                         Str(lb.Id),
		"display_name":               Str(lb.DisplayName),
		"compartment_id":             Str(lb.CompartmentId),
		"lifecycle_state":            string(lb.LifecycleState),
		"shape_name":                 Str(lb.ShapeName),
		"is_private":                 lb.IsPrivate != nil && *lb.IsPrivate,
		"ip_addresses":               ips,
		"subnet_ids":                 lb.SubnetIds,
		"network_security_group_ids": lb.NetworkSecurityGroupIds,
		"backend_set_names":          backendSets,
		"listener_names":             listeners,
		"time_created":               FormatTime(lb.TimeCreated),
	}
}

func SummariseBackendSet(bs *loadbalancer.BackendSet) map[string]interface{} {
	backends := make([]map[string]interface{}, 0, len(bs.Backends))
	for i := range bs.Backends {
		backends = append(backends, SummariseBackend(&bs.Backends[i]))
	}
	m := map[string]interface{}{
		"name":     Str(bs.Name),
		"policy":   Str(bs.Policy),
		"backends": backends,
	}
	if bs.HealthChecker != nil {
		m["health_checker"] = SummariseHealthChecker(bs.HealthChecker)
	}
	return m
}

func SummariseBackend(b *loadbalancer.Backend) map[string]interface{} {
	m := map[string]interface{}{
		"name":       Str(b.Name),
		"ip_address": Str(b.IpAddress),
		"drain":      b.Drain != nil && *b.Drain,
		"backup":     b.Backup != nil && *b.Backup,
		"offline":    b.Offline != nil && *b.Offline,
	}
	if b.Port != nil {
		m["port"] = *b.Port
	}
	if b.Weight != nil {
		m["weight"] = *b.Weight
	}
	return m
}

func SummariseListener(l *loadbalancer.Listener) map[string]interface{} {
	m := map[string]interface{}{
		"name":                     Str(l.Name),
		"default_backend_set_name": Str(l.DefaultBackendSetName),
		"protocol":                 Str(l.Protocol),
		"hostname_names":           l.HostnameNames,
		"path_route_set_name":      Str(l.PathRouteSetName),
	}
	if l.Port != nil {
		m["port"] = *l.Port
	}
	return m
}

func SummariseCertificate(c *loadbalancer.Certificate) map[string]interface{} {
	return map[string]interface{}{
		"certificate_name":   Str(c.CertificateName),
		"public_certificate": Str(c.PublicCertificate),
		"ca_certificate":     Str(c.CaCertificate),
	}
}

func SummariseHostname(h *loadbalancer.Hostname) map[string]interface{} {
	return map[string]interface{}{
		"name":     Str(h.Name),
		"hostname": Str(h.Hostname),
	}
}

func SummarisePathRouteSet(p *loadbalancer.PathRouteSet) map[string]interface{} {
	return map[string]interface{}{
		"name":        Str(p.Name),
		"path_routes": p.PathRoutes,
	}
}

func SummariseRuleSet(r *loadbalancer.RuleSet) map[string]interface{} {
	return map[string]interface{}{
		"name":  Str(r.Name),
		"items": r.Items,
	}
}

func SummariseRoutingPolicy(r *loadbalancer.RoutingPolicy) map[string]interface{} {
	return map[string]interface{}{
		"name":                       Str(r.Name),
		"condition_language_version": string(r.ConditionLanguageVersion),
		"rules":                      r.Rules,
	}
}

func SummariseSslCipherSuite(s *loadbalancer.SslCipherSuite) map[string]interface{} {
	return map[string]interface{}{
		"name":    Str(s.Name),
		"ciphers": s.Ciphers,
	}
}

func SummariseHealthChecker(h *loadbalancer.HealthChecker) map[string]interface{} {
	m := map[string]interface{}{
		"protocol": Str(h.Protocol),
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
	return m
}

func SummariseWorkRequest(w *loadbalancer.WorkRequest) map[string]interface{} {
	return map[string]interface{}{
		"id":               Str(w.Id),
		"type":             Str(w.Type),
		"load_balancer_id": Str(w.LoadBalancerId),
		"compartment_id":   Str(w.CompartmentId),
		"lifecycle_state":  string(w.LifecycleState),
		"message":          Str(w.Message),
		"time_accepted":    FormatTime(w.TimeAccepted),
		"time_finished":    FormatTime(w.TimeFinished),
	}
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

// AsyncResult is the standard envelope for the many work-request-only operations:
// it reports the work-request id the operator can poll with Get Work Request.
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

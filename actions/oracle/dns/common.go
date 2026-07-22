// Package dns holds what every Oracle Cloud (OCI) DNS action shares: the API-signing-
// key credential, the DnsClient factory, the two per-scope client preambles, the
// resource summarisers, and the input/result helpers. Like the sibling OCI packages it
// has no Execute function, so the manifest generator skips it — but its category.go
// supplies the "DNS" sub-group.
//
// Two DNS-specific quirks the actions lean on: (1) zones (and their records) are keyed
// by a ZONE-NAME-OR-OCID path segment — an operator can pass either the FQDN or the
// zone OCID; (2) almost every zone/record call takes an optional GLOBAL/PRIVATE SCOPE
// (public internet DNS vs private DNS). The DNS API is largely SYNCHRONOUS — record
// reads/updates return the record collection directly; only a few operations (zone
// create/delete) return a work-request id, surfaced via AsyncResult.
package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/dns"

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

// Scopes are the valid values for the GLOBAL/PRIVATE scope input.
var Scopes = []string{"GLOBAL", "PRIVATE"}

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

func (a Auth) DnsClient() (dns.DnsClient, error) {
	return dns.NewDnsClientWithConfigurationProvider(a.provider)
}

// Client is the preamble for compartment-scoped ops (create, list).
func Client(inputs []*coreflow.Connection) (auth Auth, client dns.DnsClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, dns.DnsClient{}, ErrorResult(err.Error())
	}
	c, err := a.DnsClient()
	if err != nil {
		return Auth{}, dns.DnsClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// ResourceClient additionally reads one resource identifier (named by inputName) — a
// zone name-or-OCID for zone/record actions, or a resource OCID for the others.
func ResourceClient(inputs []*coreflow.Connection, inputName string) (auth Auth, client dns.DnsClient, id string, errResult map[string]interface{}) {
	a, c, errRes := Client(inputs)
	if errRes != nil {
		return Auth{}, dns.DnsClient{}, "", errRes
	}
	v, err := RequiredString(inputName, inputs)
	if err != nil {
		return Auth{}, dns.DnsClient{}, "", ErrorResult(err.Error())
	}
	return a, c, v, nil
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

// OptionalScope reads and validates the GLOBAL/PRIVATE scope input, upper-casing it.
// Returns ("", nil) when blank. Each DNS request has its own scope enum type, so the
// caller casts the returned string to e.g. dns.GetZoneScopeEnum(scope).
func OptionalScope(inputs []*coreflow.Connection) (string, error) {
	raw := strings.ToUpper(strings.TrimSpace(OptionalString("scope", inputs)))
	if raw == "" {
		return "", nil
	}
	for _, s := range Scopes {
		if raw == s {
			return raw, nil
		}
	}
	return "", fmt.Errorf("scope must be GLOBAL or PRIVATE")
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

func SummariseZone(z *dns.Zone) map[string]interface{} {
	m := map[string]interface{}{
		"id":              Str(z.Id),
		"name":            Str(z.Name),
		"compartment_id":  Str(z.CompartmentId),
		"zone_type":       string(z.ZoneType),
		"lifecycle_state": string(z.LifecycleState),
		"scope":           string(z.Scope),
		"self":            Str(z.Self),
		"version":         Str(z.Version),
		"is_protected":    z.IsProtected != nil && *z.IsProtected,
		"time_created":    FormatTime(z.TimeCreated),
	}
	if z.Serial != nil {
		m["serial"] = *z.Serial
	}
	return m
}

func SummariseZoneSummary(z *dns.ZoneSummary) map[string]interface{} {
	m := map[string]interface{}{
		"id":              Str(z.Id),
		"name":            Str(z.Name),
		"compartment_id":  Str(z.CompartmentId),
		"zone_type":       string(z.ZoneType),
		"lifecycle_state": string(z.LifecycleState),
		"scope":           string(z.Scope),
		"self":            Str(z.Self),
		"is_protected":    z.IsProtected != nil && *z.IsProtected,
		"time_created":    FormatTime(z.TimeCreated),
	}
	if z.Serial != nil {
		m["serial"] = *z.Serial
	}
	return m
}

func SummariseRecord(r *dns.Record) map[string]interface{} {
	m := map[string]interface{}{
		"domain":      Str(r.Domain),
		"rtype":       Str(r.Rtype),
		"rdata":       Str(r.Rdata),
		"record_hash": Str(r.RecordHash),
	}
	if r.Ttl != nil {
		m["ttl"] = *r.Ttl
	}
	return m
}

// SummariseRecords flattens a slice of records (the RecordCollection body).
func SummariseRecords(items []dns.Record) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(items))
	for i := range items {
		out = append(out, SummariseRecord(&items[i]))
	}
	return out
}

func SummariseSteeringPolicy(p *dns.SteeringPolicy) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(p.Id),
		"display_name":    Str(p.DisplayName),
		"compartment_id":  Str(p.CompartmentId),
		"lifecycle_state": string(p.LifecycleState),
		"template":        string(p.Template),
		"ttl":             p.Ttl,
		"time_created":    FormatTime(p.TimeCreated),
	}
}

func SummariseSteeringPolicyAttachment(a *dns.SteeringPolicyAttachment) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(a.Id),
		"display_name":       Str(a.DisplayName),
		"steering_policy_id": Str(a.SteeringPolicyId),
		"zone_id":            Str(a.ZoneId),
		"domain_name":        Str(a.DomainName),
		"compartment_id":     Str(a.CompartmentId),
		"lifecycle_state":    string(a.LifecycleState),
		"time_created":       FormatTime(a.TimeCreated),
	}
}

func SummariseView(v *dns.View) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(v.Id),
		"display_name":    Str(v.DisplayName),
		"compartment_id":  Str(v.CompartmentId),
		"lifecycle_state": string(v.LifecycleState),
		"is_protected":    v.IsProtected != nil && *v.IsProtected,
		"time_created":    FormatTime(v.TimeCreated),
	}
}

func SummariseResolver(r *dns.Resolver) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(r.Id),
		"display_name":    Str(r.DisplayName),
		"compartment_id":  Str(r.CompartmentId),
		"lifecycle_state": string(r.LifecycleState),
		"is_protected":    r.IsProtected != nil && *r.IsProtected,
		"time_created":    FormatTime(r.TimeCreated),
	}
}

func SummariseResolverEndpoint(e dns.ResolverEndpoint) map[string]interface{} {
	if e == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"name":            Str(e.GetName()),
		"resolver_id":     Str(e.GetResolverId()),
		"compartment_id":  Str(e.GetCompartmentId()),
		"lifecycle_state": string(e.GetLifecycleState()),
		"is_forwarding":   e.GetIsForwarding() != nil && *e.GetIsForwarding(),
		"is_listening":    e.GetIsListening() != nil && *e.GetIsListening(),
		"self":            Str(e.GetSelf()),
		"time_created":    FormatTime(e.GetTimeCreated()),
	}
}

func SummariseTsigKey(k *dns.TsigKey) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(k.Id),
		"name":            Str(k.Name),
		"algorithm":       Str(k.Algorithm),
		"compartment_id":  Str(k.CompartmentId),
		"lifecycle_state": string(k.LifecycleState),
		"self":            Str(k.Self),
		"time_created":    FormatTime(k.TimeCreated),
	}
}

// ---------------------------------------------------------------------------
// List-summary summarisers (the *Summary structs returned by the list calls).
// Fields mirror the full summarisers above so list/get shapes stay consistent.
// ---------------------------------------------------------------------------

func SummariseViewSummary(v *dns.ViewSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(v.Id),
		"display_name":    Str(v.DisplayName),
		"compartment_id":  Str(v.CompartmentId),
		"lifecycle_state": string(v.LifecycleState),
		"is_protected":    v.IsProtected != nil && *v.IsProtected,
		"self":            Str(v.Self),
		"time_created":    FormatTime(v.TimeCreated),
	}
}

func SummariseResolverSummary(r *dns.ResolverSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(r.Id),
		"display_name":    Str(r.DisplayName),
		"compartment_id":  Str(r.CompartmentId),
		"lifecycle_state": string(r.LifecycleState),
		"is_protected":    r.IsProtected != nil && *r.IsProtected,
		"self":            Str(r.Self),
		"time_created":    FormatTime(r.TimeCreated),
	}
}

// SummariseResolverEndpointSummary handles the polymorphic ResolverEndpointSummary
// interface (the list-call item type), reading it through its getters.
func SummariseResolverEndpointSummary(e dns.ResolverEndpointSummary) map[string]interface{} {
	if e == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":                 Str(e.GetId()),
		"name":               Str(e.GetName()),
		"resolver_id":        Str(e.GetResolverId()),
		"compartment_id":     Str(e.GetCompartmentId()),
		"lifecycle_state":    string(e.GetLifecycleState()),
		"is_forwarding":      e.GetIsForwarding() != nil && *e.GetIsForwarding(),
		"is_listening":       e.GetIsListening() != nil && *e.GetIsListening(),
		"forwarding_address": Str(e.GetForwardingAddress()),
		"listening_address":  Str(e.GetListeningAddress()),
		"self":               Str(e.GetSelf()),
		"time_created":       FormatTime(e.GetTimeCreated()),
	}
}

func SummariseTsigKeySummary(k *dns.TsigKeySummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(k.Id),
		"name":            Str(k.Name),
		"algorithm":       Str(k.Algorithm),
		"compartment_id":  Str(k.CompartmentId),
		"lifecycle_state": string(k.LifecycleState),
		"self":            Str(k.Self),
		"time_created":    FormatTime(k.TimeCreated),
	}
}

func SummariseSteeringPolicySummary(p *dns.SteeringPolicySummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(p.Id),
		"display_name":    Str(p.DisplayName),
		"compartment_id":  Str(p.CompartmentId),
		"lifecycle_state": string(p.LifecycleState),
		"template":        string(p.Template),
		"ttl":             p.Ttl,
		"self":            Str(p.Self),
		"time_created":    FormatTime(p.TimeCreated),
	}
}

func SummariseSteeringPolicyAttachmentSummary(a *dns.SteeringPolicyAttachmentSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(a.Id),
		"display_name":       Str(a.DisplayName),
		"steering_policy_id": Str(a.SteeringPolicyId),
		"zone_id":            Str(a.ZoneId),
		"domain_name":        Str(a.DomainName),
		"rtypes":             a.Rtypes,
		"compartment_id":     Str(a.CompartmentId),
		"lifecycle_state":    string(a.LifecycleState),
		"self":               Str(a.Self),
		"time_created":       FormatTime(a.TimeCreated),
	}
}

// ResolverEndpointName reads the fully-qualified resolver-endpoint identity used by the
// endpoint get/update/delete calls: the parent resolver OCID plus the endpoint name.
func ResolverEndpointName(inputs []*coreflow.Connection) (resolverID, name string, err error) {
	resolverID, err = RequiredString("resolver_ocid", inputs)
	if err != nil {
		return "", "", err
	}
	name, err = RequiredString("endpoint_name", inputs)
	if err != nil {
		return "", "", err
	}
	return resolverID, name, nil
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

// AsyncResult is the envelope for the few work-request-driven DNS operations.
func AsyncResult(msg, workRequestID string) map[string]interface{} {
	return map[string]interface{}{"tool_result": msg, "work_request_id": workRequestID, "success": true}
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

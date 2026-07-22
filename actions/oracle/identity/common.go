// Package identity holds what every Oracle Cloud (OCI) Identity/IAM action shares: the
// API-signing-key credential, the IdentityClient factory, the two client preambles, the
// resource summarisers, and the input/result helpers. Like the sibling OCI packages it
// has no Execute function, so the manifest generator skips it — but its category.go
// supplies the "Identity" sub-group.
//
// IAM quirks the actions lean on: (1) Identity is a GLOBAL service anchored to the
// tenancy's HOME region — writes must target the home region, so a wrong region surfaces
// a clean OCI error rather than silently succeeding; (2) users, groups, policies and
// dynamic groups live in the TENANCY (root compartment), so CompartmentOrTenancy lets an
// action default a blank compartment to the tenancy OCID; (3) most operations are
// synchronous, but a few (compartment delete/move, bulk actions, cascade tag-namespace
// delete) return a work-request id, surfaced via AsyncResult.
package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"

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

func (a Auth) IdentityClient() (identity.IdentityClient, error) {
	return identity.NewIdentityClientWithConfigurationProvider(a.provider)
}

// Client is the preamble for compartment-scoped ops (create, list).
func Client(inputs []*coreflow.Connection) (auth Auth, client identity.IdentityClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, identity.IdentityClient{}, ErrorResult(err.Error())
	}
	c, err := a.IdentityClient()
	if err != nil {
		return Auth{}, identity.IdentityClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// ResourceClient additionally reads one resource identifier (named by inputName) — a
// user/group/policy/compartment OCID for the per-resource actions.
func ResourceClient(inputs []*coreflow.Connection, inputName string) (auth Auth, client identity.IdentityClient, id string, errResult map[string]interface{}) {
	a, c, errRes := Client(inputs)
	if errRes != nil {
		return Auth{}, identity.IdentityClient{}, "", errRes
	}
	v, err := RequiredString(inputName, inputs)
	if err != nil {
		return Auth{}, identity.IdentityClient{}, "", ErrorResult(err.Error())
	}
	return a, c, v, nil
}

func (a Auth) RequiredCompartment() (string, error) {
	if a.CompartmentOCID == "" {
		return "", fmt.Errorf("compartment OCID is required")
	}
	return a.CompartmentOCID, nil
}

// CompartmentOrTenancy returns the supplied compartment, or the tenancy OCID (root) when
// blank — the common default for IAM resources (users/groups/policies/dynamic groups
// live in the tenancy).
func (a Auth) CompartmentOrTenancy() string {
	if a.CompartmentOCID != "" {
		return a.CompartmentOCID
	}
	return a.TenancyOCID
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

// InputLines splits a newline-or-comma-separated input into a trimmed, non-empty slice —
// used for policy statements (one per line).
func InputLines(name string, inputs []*coreflow.Connection) []string {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil
	}
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	var out []string
	for _, part := range strings.Split(raw, "\n") {
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

func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// IntOrNil returns the dereferenced int, or nil when the pointer is unset — so a result
// map surfaces a genuinely-absent value as null rather than a misleading zero.
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
// Resource summarisers (core resources; the long-tail types build maps inline)
// ---------------------------------------------------------------------------

func SummariseUser(u *identity.User) map[string]interface{} {
	return map[string]interface{}{
		"id":               Str(u.Id),
		"name":             Str(u.Name),
		"description":      Str(u.Description),
		"compartment_id":   Str(u.CompartmentId),
		"email":            Str(u.Email),
		"lifecycle_state":  string(u.LifecycleState),
		"is_mfa_activated": u.IsMfaActivated != nil && *u.IsMfaActivated,
		"time_created":     FormatTime(u.TimeCreated),
	}
}

func SummariseGroup(g *identity.Group) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(g.Id),
		"name":            Str(g.Name),
		"description":     Str(g.Description),
		"compartment_id":  Str(g.CompartmentId),
		"lifecycle_state": string(g.LifecycleState),
		"time_created":    FormatTime(g.TimeCreated),
	}
}

func SummarisePolicy(p *identity.Policy) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(p.Id),
		"name":            Str(p.Name),
		"description":     Str(p.Description),
		"compartment_id":  Str(p.CompartmentId),
		"statements":      p.Statements,
		"lifecycle_state": string(p.LifecycleState),
		"time_created":    FormatTime(p.TimeCreated),
	}
}

func SummariseCompartment(c *identity.Compartment) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(c.Id),
		"name":            Str(c.Name),
		"description":     Str(c.Description),
		"compartment_id":  Str(c.CompartmentId),
		"lifecycle_state": string(c.LifecycleState),
		"is_accessible":   c.IsAccessible != nil && *c.IsAccessible,
		"time_created":    FormatTime(c.TimeCreated),
	}
}

func SummariseDynamicGroup(d *identity.DynamicGroup) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(d.Id),
		"name":            Str(d.Name),
		"description":     Str(d.Description),
		"compartment_id":  Str(d.CompartmentId),
		"matching_rule":   Str(d.MatchingRule),
		"lifecycle_state": string(d.LifecycleState),
		"time_created":    FormatTime(d.TimeCreated),
	}
}

// SummariseAuthToken deliberately omits the token secret except on create (the only
// call that returns it) — callers add "token" themselves from resp.Token.
func SummariseAuthToken(t *identity.AuthToken) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(t.Id),
		"user_id":         Str(t.UserId),
		"description":     Str(t.Description),
		"lifecycle_state": string(t.LifecycleState),
		"time_created":    FormatTime(t.TimeCreated),
		"time_expires":    FormatTime(t.TimeExpires),
	}
}

func SummariseApiKey(k *identity.ApiKey) map[string]interface{} {
	return map[string]interface{}{
		"key_id":          Str(k.KeyId),
		"fingerprint":     Str(k.Fingerprint),
		"user_id":         Str(k.UserId),
		"lifecycle_state": string(k.LifecycleState),
		"time_created":    FormatTime(k.TimeCreated),
	}
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

// Result is the standard success envelope: tool_result + success plus any extra keys.
func Result(msg string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{"tool_result": msg, "success": true}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// AsyncResult is the envelope for the few work-request-driven IAM operations.
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

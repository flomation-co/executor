// Package compute holds what every Oracle Cloud (OCI) Compute action shares: the
// API-signing-key credential and the OCI service-client factories. Like the
// azure/compute and aws packages it has no Execute function, so the manifest
// generator skips it — but its category.go still supplies the "Compute"
// sub-category metadata.
//
// OCI auth is NOT a static secret or an OAuth bearer: every request is signed
// with an RSA private key. The operator supplies the tenancy + user OCIDs, the
// region, the key fingerprint and the private-key PEM; the OCI SDK's
// NewRawConfigurationProvider turns those into a ConfigurationProvider that owns
// the request-signing pipeline, and each service client (Compute, Network,
// Identity, Blockstorage) reads its region + signer from it. GetAuth centralises
// that so the ~dozen instance/shape/image/networking actions don't each
// re-implement it. As with azure/compute, the manifest generator only resolves
// INLINE Inputs literals, so the credential + scope input *declarations*
// (tenancy_ocid, compartment_ocid, ...) must still be copy-pasted into each
// action's Inputs — only the Execute-side logic is shared here.
//
// The signing key is Oracle's own SDK primitive, not hand-rolled: the raw
// provider parses the PEM and performs the HTTP Signature signing itself, so no
// key material or signing logic lives in this repo beyond passing the PEM
// through.
package compute

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"

	coreflow "flomation.app/automate/executor"
)

// Standard input names shared by every OCI Compute action.
const (
	InputTenancyOCID     = "tenancy_ocid"
	InputUserOCID        = "user_ocid"
	InputRegion          = "region"
	InputFingerprint     = "fingerprint"
	InputPrivateKey      = "private_key"
	InputPassphrase      = "private_key_passphrase"
	InputCompartmentOCID = "compartment_ocid"
)

// Auth carries the API-signing-key material plus the compartment scope OCI list
// and create calls need. The parsed ConfigurationProvider owns the signer.
type Auth struct {
	TenancyOCID     string
	UserOCID        string
	Region          string
	Fingerprint     string
	privateKey      string // PEM — never echoed into an output or log (see redact)
	passphrase      string // optional; "" means the key is unencrypted
	CompartmentOCID string // required by list/create actions; per-instance ops use the instance OCID
	provider        common.ConfigurationProvider
}

// GetAuth reads the standard credential + scope input block and builds the
// signing-key ConfigurationProvider. CompartmentOCID is read but not required
// here — actions that need it call RequiredCompartment so the error names the
// field the operator left blank. The private key is validated eagerly (a bad PEM
// is the most common misconfig) so the failure is a clean message, not a signing
// panic deep in the first API call.
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
	// The region string SELECTS the OCI API host: the SDK builds
	// https://<service>.<region>.<realm> from it, and a region containing a dot
	// short-circuits the realm suffix so every signed request would go to an
	// arbitrary https://<service>.<attacker-host>. Constrain it to a plain region
	// label before it reaches the SDK — the analogue of the Azure node's guard on
	// its host-selecting tenant field. Legitimate OCI regions never contain a dot.
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

	// Parse the key now so a malformed/mis-pasted PEM fails with a clear message
	// up front rather than as an opaque signing error on the first real call.
	if _, err := a.provider.PrivateRSAKey(); err != nil {
		return Auth{}, fmt.Errorf("private key could not be parsed — check it is the full PEM (and the passphrase, if set): %s", a.redact(err.Error()))
	}
	return a, nil
}

// ---------------------------------------------------------------------------
// OCI service-client factory — one authenticated client per service
// ---------------------------------------------------------------------------

func (a Auth) ComputeClient() (core.ComputeClient, error) {
	return core.NewComputeClientWithConfigurationProvider(a.provider)
}

func (a Auth) NetworkClient() (core.VirtualNetworkClient, error) {
	return core.NewVirtualNetworkClientWithConfigurationProvider(a.provider)
}

func (a Auth) BlockstorageClient() (core.BlockstorageClient, error) {
	return core.NewBlockstorageClientWithConfigurationProvider(a.provider)
}

func (a Auth) IdentityClient() (identity.IdentityClient, error) {
	return identity.NewIdentityClientWithConfigurationProvider(a.provider)
}

// PerInstanceClient reads the credential block + the instance_ocid input and
// builds a Compute client — the shared preamble for every per-instance action
// (get / start / stop / reset / terminate / update). On any setup error it
// returns a ready ErrorResult so the caller can `return errResult, nil`; on
// success errResult is nil.
func PerInstanceClient(inputs []*coreflow.Connection) (auth Auth, client core.ComputeClient, instanceOCID string, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, core.ComputeClient{}, "", ErrorResult(err.Error())
	}
	id, err := RequiredString("instance_ocid", inputs)
	if err != nil {
		return Auth{}, core.ComputeClient{}, "", ErrorResult(err.Error())
	}
	c, err := a.ComputeClient()
	if err != nil {
		return Auth{}, core.ComputeClient{}, "", ErrorResult(a.OCIError(err))
	}
	return a, c, id, nil
}

// RequiredCompartment is the common case: nearly every list/create action is
// scoped to one compartment. Kept separate so the message reads
// "compartment OCID is required".
func (a Auth) RequiredCompartment() (string, error) {
	if a.CompartmentOCID == "" {
		return "", fmt.Errorf("compartment OCID is required")
	}
	return a.CompartmentOCID, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// OptionalString returns a string input's value, or "" when absent/unset.
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

// OptionalBool returns a boolean input's value, or def when the input is absent
// or unset. Uses Connection.Boolean(), which resolves both literal checkboxes and
// variable-bound values (see the connection-accessor fix, executor !188).
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

// validRegion constrains an OCI region to a plain identifier (e.g. uk-london-1),
// rejecting dots/slashes/colons/whitespace so a crafted value can't redirect the
// SDK's host construction. See GetAuth.
var validRegion = regexp.MustCompile(`^[a-z0-9-]+$`)

// fieldLabel turns an input name into an operator-facing label for error
// messages: underscores become spaces and the OCID token is upper-cased, so
// "instance_ocid" reads "instance OCID" (consistent with RequiredCompartment's
// "compartment OCID is required").
func fieldLabel(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "_", " "), "ocid", "OCID")
}

// RequiredString returns a trimmed input value, or an error naming the field.
func RequiredString(name string, inputs []*coreflow.Connection) (string, error) {
	if v := strings.TrimSpace(OptionalString(name, inputs)); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s is required", fieldLabel(name))
}

// OptionalFloat32 reads a decimal input (e.g. flex-shape OCPUs/memory), returning
// (value, true) when set and parseable, (0, false) when blank, or an error when
// present but not a number.
func OptionalFloat32(name string, inputs []*coreflow.Connection) (float32, bool, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return 0, false, nil
	}
	f, err := strconv.ParseFloat(raw, 32)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be a number", strings.ReplaceAll(name, "_", " "))
	}
	return float32(f), true, nil
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

// FreeformTags parses a JSON object input ({"env":"prod"}) into the map[string]string
// OCI's FreeformTags field wants. An empty/blank input yields (nil, nil);
// malformed JSON, or JSON that isn't an object of strings, is an error.
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

// SummariseInstance flattens the SDK instance into a compact, JSON-friendly map.
// Shared by instance_get_all, instance_get and the lifecycle actions so the
// "instance" output shape is identical everywhere. lifecycle_state is the OCI
// power/lifecycle state (RUNNING / STOPPED / TERMINATED / …) an operator acts on.
func SummariseInstance(inst *core.Instance) map[string]interface{} {
	m := map[string]interface{}{
		"id":                  Str(inst.Id),
		"display_name":        Str(inst.DisplayName),
		"lifecycle_state":     string(inst.LifecycleState),
		"shape":               Str(inst.Shape),
		"region":              Str(inst.Region),
		"availability_domain": Str(inst.AvailabilityDomain),
		"compartment_id":      Str(inst.CompartmentId),
	}
	if inst.TimeCreated != nil {
		m["time_created"] = inst.TimeCreated.String()
	}
	tags := map[string]string{}
	for k, v := range inst.FreeformTags {
		tags[k] = v
	}
	m["freeform_tags"] = tags
	return m
}

// Str safely dereferences an SDK *string field (OCI models are pointer-heavy),
// yielding "" for nil.
func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// StringPtr wraps a value for the SDK's mandatory pointer fields.
func StringPtr(s string) *string { return &s }

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

// ErrorResult is the soft-failure envelope: the flow continues, and the node
// reports success=false with a clean message on the error/tool_result outputs.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"success":     false,
		"error":       msg,
		"tool_result": msg,
	}
}

// OCIError turns an OCI SDK error into an operator-readable string. OCI surfaces
// failures as a common.ServiceError carrying the service code (e.g. NotAuthorized,
// InstanceNotFound) and HTTP status; those name the real problem far better than
// the raw wire dump. The private key/passphrase are redacted defensively.
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

// redact strips the private key and passphrase from any string bound for an
// output or log. The PEM should never appear in an error, but a mis-signed
// request or a wrapped transport error could echo request material, so this is
// defence in depth.
func (a Auth) redact(s string) string {
	if k := strings.TrimSpace(a.privateKey); k != "" {
		s = strings.ReplaceAll(s, k, "REDACTED")
		// Also scrub the raw (untrimmed) form and any embedded PEM body lines.
		s = strings.ReplaceAll(s, a.privateKey, "REDACTED")
	}
	if a.passphrase != "" {
		s = strings.ReplaceAll(s, a.passphrase, "REDACTED")
	}
	return s
}

// firstLine keeps error strings to their first meaningful line.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// Context is what every action passes: no request deadline from the executor.
func Context() context.Context { return context.Background() }

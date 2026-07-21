// Package objectstorage holds what every Oracle Cloud (OCI) Object Storage action
// shares: the API-signing-key credential, the ObjectStorageClient factory, the
// tenancy-namespace resolver, and the input/result helpers. Like the
// oracle/compute package it has no Execute function, so the manifest generator
// skips it — but its category.go supplies the "Object Storage" sub-category.
//
// The auth block mirrors oracle/compute (same signing-key model: tenancy/user
// OCID, region, fingerprint, private-key PEM, optional passphrase). It is
// intentionally self-contained per sub-group for now — a shared oracle-level auth
// package is a worthwhile refactor once Track A has a few services, but keeping it
// here avoids touching the already-shipped compute node.
package objectstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"

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

// ListMaxPages bounds every list action's pagination walk so one node run can't
// turn into an unbounded sequence of API calls; a walk that hits the cap sets
// truncated=true so a capped result is distinguishable from a complete one.
const ListMaxPages = 25

// validRegion constrains the host-selecting region to a plain label (see GetAuth).
var validRegion = regexp.MustCompile(`^[a-z0-9-]+$`)

// Auth carries the API-signing-key material plus the compartment scope that
// bucket create/list calls need. Object calls are scoped by namespace + bucket.
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
// ConfigurationProvider, validating the region (host-selecting) and eagerly
// parsing the PEM so a bad key fails with a clean message up front.
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

// Client builds an authenticated Object Storage client.
func (a Auth) Client() (objectstorage.ObjectStorageClient, error) {
	return objectstorage.NewObjectStorageClientWithConfigurationProvider(a.provider)
}

// Namespace resolves the tenancy's Object Storage namespace (a single call, the
// value is stable per tenancy). Actions call this so the operator never has to
// know or paste it.
func (a Auth) Namespace(ctx context.Context, client objectstorage.ObjectStorageClient) (string, error) {
	resp, err := client.GetNamespace(ctx, objectstorage.GetNamespaceRequest{})
	if err != nil {
		return "", err
	}
	return Str(resp.Value), nil
}

// RequiredCompartment names the field when a compartment-scoped action is missing it.
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

func RequiredString(name string, inputs []*coreflow.Connection) (string, error) {
	if v := strings.TrimSpace(OptionalString(name, inputs)); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s is required", strings.ReplaceAll(name, "_", " "))
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

// OptionalInt reads a whole-number input, returning def when blank and an error
// when present but not an integer.
func OptionalInt(name string, inputs []*coreflow.Connection, def int) (int, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def, fmt.Errorf("%s must be a whole number", strings.ReplaceAll(name, "_", " "))
	}
	return n, nil
}

// StringMap parses a JSON object input into map[string]string; blank → nil. The
// label names the field in the error message (e.g. "tags", "metadata").
func StringMap(name, label string, inputs []*coreflow.Connection) (map[string]string, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var flat map[string]string
	if err := json.Unmarshal([]byte(raw), &flat); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object of string values, e.g. {\"key\":\"value\"}: %s", label, err.Error())
	}
	return flat, nil
}

// FreeformTags parses a JSON object input into map[string]string; blank → nil.
func FreeformTags(name string, inputs []*coreflow.Connection) (map[string]string, error) {
	return StringMap(name, "tags", inputs)
}

func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func StringPtr(s string) *string { return &s }

// FormatTime renders an OCI SDKTime as RFC3339 (UTC) so every action emits
// timestamps in one machine-parseable format; blank for a nil time.
func FormatTime(t *common.SDKTime) string {
	if t == nil {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{"success": false, "error": msg, "tool_result": msg}
}

// ServiceErrorCode returns the OCI service error code (e.g. "IfNoneMatchFailed")
// and HTTP status for err, or ("", 0) when err is not an OCI service error. Use
// it to give a targeted message on a specific code before falling back to OCIError.
func ServiceErrorCode(err error) (string, int) {
	if se, ok := common.IsServiceError(err); ok {
		return se.GetCode(), se.GetHTTPStatusCode()
	}
	return "", 0
}

// OCIError summarises an OCI SDK error, redacting the key/passphrase defensively.
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

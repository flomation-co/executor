// Package storage holds the shared auth resolution, input helpers, validators
// and output shapers used by every azure/storage/* action.
//
// The Blob wire protocol — request signing, XML parsing, pagination — is owned
// by the azblob SDK now (see sdk.go for the client factory). This file used to
// hand-roll the SharedKey string-to-sign and parse the XML envelopes itself;
// that is gone. The SDK's signer is what a live Azurite round-trip exercises,
// and moving to it is the whole point of the migration: the 13-slot Blob
// signature is exactly the kind of detail better owned by code Microsoft
// maintains (the sibling File SAS, still hand-rolled in azure/files, shipped a
// 100%-broken link in wave 2 from a one-slot mistake).
//
// What stays here is transport-agnostic: GetAuth (which normalises BaseURL for
// both host styles — the public-cloud default https://{account}.blob.core.
// windows.net and the Azurite path style http://host:10000/{account} — so the
// SDK client is built the same way for real Azure and the emulator), the input
// parsers/validators, credential redaction, the lease-call builder, and the
// ErrorResult/ResourceResult/ListResult output envelopes.
package storage

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	azure "flomation.app/automate/executor/actions/azure"
)

const (
	// EntraScope is the client-credentials scope for the Blob service.
	EntraScope = "https://storage.azure.com/.default"

	// MaxDownloadBody caps a blob download. Blobs can be huge; 256 MB is the
	// same ceiling the synchronous copy-from-URL API imposes server-side.
	MaxDownloadBody = 256 << 20 // 256 MB

	// requestTimeout is the HTTP client timeout for a single Blob service call.
	// Generous because a download/upload of maxDownloadBody must fit inside it.
	requestTimeout = 60 * time.Second

	// DefaultPageLimit / MaxPageLimit bound the `maxresults` query param.
	DefaultPageLimit = 50
	MaxPageLimit     = 5000

	// MaxListPages bounds a return_all pagination walk so an account or container
	// with tens of millions of blobs can never spin unbounded requests. At
	// MaxPageLimit (5000) items per page this still admits a million entries. It
	// is exported and shared by every paginating list action (blob_get_all,
	// container_get_all, blob_find_by_tags) so the cap is defined once.
	MaxListPages = 200
)

// nowFunc is the clock for x-ms-date and SAS defaults; a var so the signing
// tests can pin it and assert an exact Authorization header.
var nowFunc = time.Now

// httpClient is shared across every Storage action so TLS connections to the
// account endpoint are pooled and reused rather than re-dialled per call.
// insecureHTTPClient is the same but skips TLS verification, used only when
// the action opts in via allow_insecure — a separate client so the secure
// default can never be weakened by a per-request tweak.
//
// DisableCompression is the sharp one. Content-Encoding is a STORED property
// of a blob, not a transfer encoding: Azure serves the bytes exactly as they
// were uploaded and never compresses on the fly. Left enabled, net/http adds
// its own Accept-Encoding: gzip, reads the stored "Content-Encoding: gzip" as
// its own doing, and hands back the DECOMPRESSED body with Content-Encoding
// and Content-Length stripped — so a download of a gzip-encoded blob returns
// different bytes than were uploaded, and disagrees with the Range path
// (net/http skips its gzip handling whenever a Range header is present).
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	},
}

var insecureHTTPClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // #nosec G402 — opt-in only, for self-signed custom endpoints
	},
}

// Auth method values for the auth_method dropdown. Empty reads as shared_key
// so a fresh node with the dropdown untouched authenticates with the key the
// operator just pasted.
const (
	AuthSharedKey = "shared_key"
	AuthEntra     = "entra"
)

// AuthInputs is the canonical credential block. Action packages re-declare
// their own literal Inputs arrays (the manifest generator AST-parses those
// literals, so it cannot see through a shared variable), but every action puts
// these eight first, in this order — storage_inputs_drift_test.go enforces it.
//
// The names matter. core.FindConnection returns the FIRST input whose name
// matches, and auth inputs are declared first — so a resource field that
// reuses one of these names silently reads the credential instead. account_name,
// account_key, endpoint and the azure_* names are therefore reserved.
var AuthInputs = []core.Connection{
	{
		Name:        "account_name",
		Type:        core.ConnectionTypeString,
		Label:       "Storage Account",
		Placeholder: "mystorageaccount",
		Required:    true,
	},
	{
		Name:  "auth_method",
		Type:  core.ConnectionTypeString,
		Label: "Authentication",
		Options: []core.ConnectionOption{
			{Name: "Shared Key", Value: AuthSharedKey},
			{Name: "Microsoft Entra (service principal)", Value: AuthEntra},
		},
	},
	{
		Name:        "account_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "Account Key",
		Placeholder: "Base64 account key — Storage Account ▸ Access keys",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"", AuthSharedKey}},
	},
	{
		Name:        "azure_tenant_id",
		Type:        core.ConnectionTypeString,
		Label:       "Tenant ID",
		Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthEntra}},
	},
	{
		Name:        "azure_client_id",
		Type:        core.ConnectionTypeString,
		Label:       "Client ID",
		Placeholder: "Application (client) ID of the service principal",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthEntra}},
	},
	{
		Name:        "azure_client_secret",
		Type:        core.ConnectionTypeSecret,
		Label:       "Client Secret",
		Placeholder: "The app needs a Storage Blob Data role on the account (RBAC)",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthEntra}},
	},
	{
		Name:        "endpoint",
		Type:        core.ConnectionTypeString,
		Label:       "Custom Endpoint",
		Placeholder: "https://myaccount.blob.core.windows.net — leave blank to derive; Azurite: http://host:10000/devstoreaccount1",
	},
	{
		Name:        "allow_insecure",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Allow Insecure TLS",
		Placeholder: "Skip TLS verification — only for custom endpoints with a self-signed certificate",
	},
}

// Auth is the resolved connection: the account name (used in the signature
// even when a custom endpoint carries it in the path), the chosen method with
// its credentials, and the normalised base URL (scheme + host [+ account path
// for Azurite-style endpoints], no trailing slash).
type Auth struct {
	AccountName  string
	Method       string
	AccountKey   []byte // decoded shared key; nil under Entra
	rawKey       string // as pasted, for redaction
	TenantID     string
	ClientID     string
	ClientSecret string
	BaseURL      string
	Insecure     bool
}

// accountNameRe is the Azure storage-account charset (lowercase alphanumeric,
// 3-24 chars — Azurite's devstoreaccount1 fits). Enforced because the name is
// interpolated into a host and into the canonicalized resource.
var accountNameRe = regexp.MustCompile(`^[a-z0-9]{3,24}$`)

// GetAuth resolves the credential block. Errors are user-configuration
// problems, so callers surface them as soft failures (ErrorResult).
func GetAuth(inputs []*core.Connection) (Auth, error) {
	account, err := RequiredString("account_name", inputs)
	if err != nil {
		return Auth{}, err
	}
	account = strings.ToLower(account)
	if !accountNameRe.MatchString(account) {
		return Auth{}, fmt.Errorf("account_name %q is not a valid storage account name (3-24 lowercase letters and digits)", account)
	}

	a := Auth{
		AccountName: account,
		Method:      OptionalString("auth_method", inputs),
		Insecure:    OptionalBool("allow_insecure", inputs),
	}
	if a.Method == "" {
		a.Method = AuthSharedKey
	}

	switch a.Method {
	case AuthSharedKey:
		rawKey, err := RequiredString("account_key", inputs)
		if err != nil {
			return Auth{}, err
		}
		// Validate the base64 here for a friendlier message than the SDK's; the
		// SDK re-decodes it when the credential is built, so the bytes are not
		// stored — a.rawKey carries the pasted string to SharedKeyCredential.
		if _, err := base64.StdEncoding.DecodeString(rawKey); err != nil {
			return Auth{}, fmt.Errorf("account_key is not valid base64 — paste the key exactly as shown under Access keys")
		}
		a.rawKey = rawKey
	case AuthEntra:
		if a.TenantID, err = RequiredString("azure_tenant_id", inputs); err != nil {
			return Auth{}, err
		}
		if a.ClientID, err = RequiredString("azure_client_id", inputs); err != nil {
			return Auth{}, err
		}
		if a.ClientSecret, err = RequiredString("azure_client_secret", inputs); err != nil {
			return Auth{}, err
		}
	default:
		return Auth{}, fmt.Errorf("auth_method %q is not supported (use shared_key or entra)", a.Method)
	}

	endpoint := OptionalString("endpoint", inputs)
	if endpoint == "" {
		a.BaseURL = "https://" + account + ".blob.core.windows.net"
	} else {
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Auth{}, fmt.Errorf("endpoint must be an http(s) URL, e.g. https://myaccount.blob.core.windows.net")
		}
		a.BaseURL = strings.TrimRight(endpoint, "/")
	}
	return a, nil
}

// ---------------------------------------------------------------------------
// Redaction
// ---------------------------------------------------------------------------

// sasSigRe matches a SAS signature query value so a URL echoed into an error
// (e.g. by a transport failure on a copy-from-URL source) never leaks it.
var sasSigRe = regexp.MustCompile(`(sig=)[^&\s"']+`)

// redact scrubs credential material from an error string: SAS signatures in
// URLs and Authorization-style values. Auth-aware masking of the literal key
// and client secret is layered on top by Auth.redact.
func redact(msg string) string {
	return sasSigRe.ReplaceAllString(msg, "${1}REDACTED")
}

// RedactURL scrubs the SAS signature out of a URL that is about to be echoed
// into an action's OUTPUT, where errors are not the only leak: result objects
// are persisted in the run record and forwarded to every downstream node.
// Only sig= is a credential — the rest of a SAS (sv/sp/se) and any
// snapshot/versionid identify WHICH source was read, which is provenance worth
// keeping.
func RedactURL(raw string) string {
	return redact(raw)
}

// redact masks this connection's own secrets in addition to the generic
// patterns. Every error string that could contain transport detail is passed
// through here before it reaches an output.
func (a Auth) redact(msg string) string {
	if a.rawKey != "" {
		msg = azure.RedactSecret(msg, a.rawKey)
	}
	if a.ClientSecret != "" {
		msg = azure.RedactSecret(msg, a.ClientSecret)
	}
	return redact(msg)
}

// ---------------------------------------------------------------------------
// Shared Key signing
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Paths & names
// ---------------------------------------------------------------------------

// containerNameRe: lowercase letters/digits/hyphens, starting and ending
// alphanumeric. Length (3-63) and consecutive hyphens are checked separately
// (the class can't express either cleanly).
var containerNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// ValidateContainerName enforces the service's container naming rules
// client-side so the operator gets a readable message instead of a signed
// request that 400s.
func ValidateContainerName(name string) error {
	if len(name) < 3 || len(name) > 63 || !containerNameRe.MatchString(name) || strings.Contains(name, "--") {
		return fmt.Errorf("container name %q is invalid: 3-63 lowercase letters, digits and hyphens, starting and ending with a letter or digit, no consecutive hyphens", name)
	}
	return nil
}

// ContainerPath returns the escaped logical path for a container.
func ContainerPath(container string) string {
	return "/" + url.PathEscape(container)
}

// BlobPath returns the escaped logical path for a blob. Blob names may
// contain "/" as a virtual-directory separator, so each segment is escaped
// individually — a name with a space or # signs correctly (n8n interpolates
// names raw and breaks on both).
func BlobPath(container, blob string) string {
	segs := strings.Split(blob, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return ContainerPath(container) + "/" + strings.Join(segs, "/")
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// OptionalString extracts a string input, returning "" if absent.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}

// RequiredString extracts a required string input, erroring if absent/blank.
func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// OptionalInt extracts an integer input. The bool is false when absent.
func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Number() == nil {
		return 0, false
	}
	return int(*conn.Number()), true
}

// OptionalBool extracts a boolean input, defaulting to false when unset.
func OptionalBool(name string, inputs []*core.Connection) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false
	}
	return *conn.Boolean()
}

// BoolDefaultTrue extracts a boolean input whose unset state means true
// (overwrite, wait_for_completion). Only an explicit false turns it off.
func BoolDefaultTrue(name string, inputs []*core.Connection) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return true
	}
	return *conn.Boolean()
}

// OptionalJSON parses an object-typed input into an arbitrary value. Returns
// (nil, nil) when absent/blank, (nil, err) on malformed JSON.
func OptionalJSON(name string, inputs []*core.Connection) (interface{}, error) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return nil, nil
	}
	switch v := conn.Value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		var out interface{}
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, fmt.Errorf("%s must be valid JSON: %w", name, err)
		}
		return out, nil
	default:
		return conn.Value, nil
	}
}

// StringMapInput parses an object input (metadata, tags) into a flat
// string→string map, coercing scalar values to strings. Returns (nil, nil)
// when the input is absent.
func StringMapInput(name string, inputs []*core.Connection) (map[string]string, error) {
	v, err := OptionalJSON(name, inputs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`%s must be a JSON object, e.g. {"key":"value"}`, name)
	}
	out := make(map[string]string, len(obj))
	for k, val := range obj {
		switch tv := val.(type) {
		case string:
			out[k] = tv
		case nil:
			out[k] = ""
		default:
			out[k] = fmt.Sprintf("%v", tv)
		}
	}
	return out, nil
}

// ClampLimit bounds a requested maxresults to the service's 1-5000 range,
// falling back to DefaultPageLimit when unset.
func ClampLimit(limit int, set bool) int {
	if !set || limit <= 0 {
		return DefaultPageLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit
	}
	return limit
}

// BlobIncludeTokens / ContainerIncludeTokens are the full sets the List Blobs
// and List Containers `include` query param accepts, in the service's own
// order. Anything outside them is a 400 InvalidQueryParameterValue that names
// nothing useful, so the tokens are checked here instead.
var (
	BlobIncludeTokens = []string{
		"copy", "deleted", "deletedwithversions", "immutabilitypolicy",
		"legalhold", "metadata", "permissions", "snapshots", "tags",
		"uncommittedblobs", "versions",
	}
	ContainerIncludeTokens = []string{"metadata", "deleted", "system"}
)

// ParseIncludeTokens turns an `include` input into the comma-separated value
// the query param takes. The input is a ComboBox — the Options shortlist covers
// the common single choices, and free text is what allows them to be COMBINED
// ("metadata,tags"), which is the only way to get both in one listing pass.
//
// Blank tokens are skipped and duplicates dropped; an unknown token is an error
// rather than something forwarded to the service.
func ParseIncludeTokens(raw string, allowed []string) (string, error) {
	valid := make(map[string]bool, len(allowed))
	for _, v := range allowed {
		valid[v] = true
	}
	out := make([]string, 0, len(allowed))
	seen := make(map[string]bool, len(allowed))
	for _, part := range strings.Split(raw, ",") {
		tok := strings.ToLower(strings.TrimSpace(part))
		if tok == "" || seen[tok] {
			continue
		}
		if !valid[tok] {
			return "", fmt.Errorf("include value %q is not supported — choose from %s, combining several with commas", tok, strings.Join(allowed, ", "))
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return strings.Join(out, ","), nil
}

// metadataNameRe: metadata names travel as x-ms-meta-{name} headers and must
// be valid C# identifiers (the service enforces this server-side with an
// opaque error, so we validate up front).
var metadataNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// tagCharRe is the blob index tag charset (both keys and values).
var tagCharRe = regexp.MustCompile(`^[a-zA-Z0-9 +\-./:=_]*$`)

// ValidateTags enforces the blob index tag rules: ≤10 tags, key 1-128 chars,
// value ≤256 chars, restricted charset.
func ValidateTags(tags map[string]string) error {
	if len(tags) > 10 {
		return fmt.Errorf("a blob can carry at most 10 index tags (got %d)", len(tags))
	}
	for k, v := range tags {
		if len(k) == 0 || len(k) > 128 || !tagCharRe.MatchString(k) {
			return fmt.Errorf("tag key %q is invalid: 1-128 chars from letters, digits and +-./:=_", k)
		}
		if len(v) > 256 || !tagCharRe.MatchString(v) {
			return fmt.Errorf("tag value for %q is invalid: up to 256 chars from letters, digits and +-./:=_", k)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Leases
// ---------------------------------------------------------------------------

// A lease is a write lock on a blob or container, held for a fixed duration
// (or indefinitely) and identified by a GUID. Two halves live here:
//
//   - LeaseIDInput / LeaseHeader — the x-ms-lease-id an EXISTING action sends
//     to prove it holds the lock. Without it, every write to a leased resource
//     is refused with 412 LeaseIdMissing. Reads are not blocked by a lease, but
//     the header is still accepted on them as an assertion: "fail unless the
//     lease is active and mine".
//   - LeaseActionOptions / BuildLeaseCall / LeaseResult — the lifecycle
//     (PUT ?comp=lease) that MINTS those IDs. It is what makes the input above
//     usable: without an acquire, a lease ID could only ever arrive from
//     outside the platform.
const (
	LeaseAcquire = "acquire"
	LeaseRenew   = "renew"
	LeaseChange  = "change"
	LeaseRelease = "release"
	LeaseBreak   = "break"
)

// Lease duration bounds. A finite lease is 15-60 seconds; -1 means "held until
// released", which is the sharp one — an infinite lease survives the flow that
// took it, so an operator who never releases it locks the blob for good (only
// a break clears it).
const (
	LeaseInfiniteDuration = -1
	leaseMinDuration      = 15
	leaseMaxDuration      = 60
	// LeaseDefaultDuration is what an unset Duration means. 60s is the longest
	// FINITE lease: the safest default, because the worst case of a flow that
	// dies holding it is a minute of waiting, not a permanently locked blob.
	LeaseDefaultDuration = 60
	leaseMaxBreakPeriod  = 60
)

// LeaseActionOptions is the lease_action dropdown, shared by blob_lease and
// container_lease so the two nodes cannot drift apart.
var LeaseActionOptions = []core.ConnectionOption{
	{Name: "Acquire — take the lock", Value: LeaseAcquire},
	{Name: "Renew — extend the lock you hold", Value: LeaseRenew},
	{Name: "Change — swap the lock's ID", Value: LeaseChange},
	{Name: "Release — hand the lock back", Value: LeaseRelease},
	{Name: "Break — end someone else's lock", Value: LeaseBreak},
}

// LeaseIDInput is the canonical optional lease-id field carried by every
// action that touches a resource somebody may have leased. Like AuthInputs it
// is documentation rather than enforcement (the manifest generator AST-parses
// each action's literal), so storage_inputs_drift_test.go compares the copies
// against it.
//
// It is deliberately NOT part of the credential block: a lease ID is an
// operator-supplied fact about one call, not a credential, so it sits with the
// resource fields and the auth-block drift assertion stays untouched.
var LeaseIDInput = core.Connection{
	Name:        "lease_id",
	Type:        core.ConnectionTypeString,
	Label:       "Lease ID",
	Placeholder: "Only needed when the blob or container is leased — the Lease ID output of a Lease step",
}

// leaseIDRe is the GUID form Azure requires of a proposed lease ID. The
// service rejects anything else with a bare 400 InvalidHeaderValue naming only
// the header, so the check is worth making here where the message can name the
// field and the rule.
var leaseIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// LeaseCall is one resolved Lease Blob / Lease Container request: the action
// chosen, the x-ms-lease-* headers it implies, and the two numbers the summary
// needs back.
type LeaseCall struct {
	Action      string
	Headers     map[string]string
	Duration    int // acquire only
	BreakPeriod int // break only; -1 when not specified
}

// BuildLeaseCall resolves the lease inputs into headers, rejecting the
// combinations the service would reject — with a message that names the field
// instead of the header. Every error is an operator-configuration problem, so
// callers surface them via ErrorResult.
func BuildLeaseCall(inputs []*core.Connection) (LeaseCall, error) {
	action, err := RequiredString("lease_action", inputs)
	if err != nil {
		return LeaseCall{}, err
	}
	action = strings.ToLower(action)

	c := LeaseCall{Action: action, BreakPeriod: -1, Headers: map[string]string{"x-ms-lease-action": action}}
	leaseID := OptionalString("lease_id", inputs)
	proposed := OptionalString("proposed_lease_id", inputs)

	switch action {
	case LeaseAcquire:
		duration, ok := OptionalInt("duration", inputs)
		if !ok {
			duration = LeaseDefaultDuration
		}
		if duration != LeaseInfiniteDuration && (duration < leaseMinDuration || duration > leaseMaxDuration) {
			return LeaseCall{}, fmt.Errorf("duration must be between %d and %d seconds, or -1 to hold the lease until it is released (got %d)",
				leaseMinDuration, leaseMaxDuration, duration)
		}
		c.Duration = duration
		c.Headers["x-ms-lease-duration"] = strconv.Itoa(duration)
		// On acquire the ID travels as x-ms-proposed-lease-id, never
		// x-ms-lease-id: there is no lease yet to name. Blank means the
		// service mints one and reports it back.
		if proposed != "" {
			if !leaseIDRe.MatchString(proposed) {
				return LeaseCall{}, fmt.Errorf("proposed_lease_id must be a GUID, e.g. 8b1c6a2e-0f9d-4a3b-9c5e-7d2f1a4b6c8d (got %q) — leave it blank to let Azure choose one", proposed)
			}
			c.Headers["x-ms-proposed-lease-id"] = proposed
		}
	case LeaseRenew, LeaseRelease:
		if leaseID == "" {
			return LeaseCall{}, fmt.Errorf("lease_id is required to %s a lease — it is the Lease ID output of the Acquire step", action)
		}
		c.Headers["x-ms-lease-id"] = leaseID
	case LeaseChange:
		if leaseID == "" {
			return LeaseCall{}, fmt.Errorf("lease_id is required to change a lease — it is the ID the lease has now")
		}
		if proposed == "" {
			return LeaseCall{}, fmt.Errorf("proposed_lease_id is required to change a lease — it is the ID the lease will have next")
		}
		if !leaseIDRe.MatchString(proposed) {
			return LeaseCall{}, fmt.Errorf("proposed_lease_id must be a GUID, e.g. 8b1c6a2e-0f9d-4a3b-9c5e-7d2f1a4b6c8d (got %q)", proposed)
		}
		c.Headers["x-ms-lease-id"] = leaseID
		c.Headers["x-ms-proposed-lease-id"] = proposed
	case LeaseBreak:
		// Break is the only action that does not need the ID: breaking a lease
		// is precisely what an operator who never had it does. Sent when known,
		// which narrows the break to that specific lease.
		if leaseID != "" {
			c.Headers["x-ms-lease-id"] = leaseID
		}
		if period, ok := OptionalInt("break_period", inputs); ok {
			if period < 0 || period > leaseMaxBreakPeriod {
				return LeaseCall{}, fmt.Errorf("break_period must be between 0 and %d seconds — 0 ends the lease immediately (got %d)", leaseMaxBreakPeriod, period)
			}
			c.BreakPeriod = period
			c.Headers["x-ms-lease-break-period"] = strconv.Itoa(period)
		}
	default:
		return LeaseCall{}, fmt.Errorf("lease_action %q is not supported (use acquire, renew, change, release or break)", action)
	}
	return c, nil
}

// ---------------------------------------------------------------------------
// XML envelopes
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Response-header parsing
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// SAS generation
// ---------------------------------------------------------------------------

// sasPermissionOrder is the canonical permission ordering for a service SAS.
// The service rejects tokens whose sp string is out of order, so we validate
// rather than silently reorder — an operator who typed "wr" should learn the
// rule, not get a token that means something else.
const sasPermissionOrder = "racwdxltmei"

// ValidateSASPermissions checks perms is a non-empty subset of
// sasPermissionOrder, in canonical order, without duplicates.
func ValidateSASPermissions(perms string) error {
	if perms == "" {
		return fmt.Errorf("permissions is required (e.g. \"r\" for read-only)")
	}
	last := -1
	for _, r := range perms {
		idx := strings.IndexRune(sasPermissionOrder, r)
		if idx < 0 {
			return fmt.Errorf("permission %q is not valid: use characters from %q", string(r), sasPermissionOrder)
		}
		if idx <= last {
			return fmt.Errorf("permissions %q are out of order or duplicated: follow the order %q", perms, sasPermissionOrder)
		}
		last = idx
	}
	return nil
}

// SASParams are the knobs of a service SAS. Container/Blob are the RAW
// (unescaped) names — the SAS string-to-sign canonicalizes the decoded path.
type SASParams struct {
	Resource           string // "b" (blob) or "c" (container)
	Container          string
	Blob               string
	Permissions        string
	Start              time.Time // zero ⇒ omitted
	Expiry             time.Time
	IP                 string
	ContentDisposition string
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ErrorResult is the standard soft-failure output map (returned with a nil
// error so the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ResourceResult shapes a single-resource response into the standard action
// output.
func ResourceResult(id string, obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          id,
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard list output.
func ListResult(items []interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

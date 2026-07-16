// Package storage holds the shared HTTP client, SharedKey/Entra auth, XML
// parsing, and pagination used by every azure/storage/* action.
//
// The Blob service REST API has three shapes worth knowing before reading on:
//
//   - Requests are signed. Shared Key auth HMAC-SHA256s a "string to sign"
//     assembled from the verb, a fixed list of standard-header slots, the
//     canonicalized x-ms-* headers, and the canonicalized resource. The slot
//     ORDER is Content-Encoding before Content-Language, per the official
//     spec — n8n's node has those two swapped (it only works there because
//     both are normally empty). Do not "fix" this file to match n8n.
//   - Responses are XML: lists arrive as an <EnumerationResults> envelope,
//     tags as <Tags><TagSet>, and errors as <Error><Code><Message>. Single
//     resources return NO body — their properties and metadata travel in
//     response headers (x-ms-*, x-ms-meta-*).
//   - Pagination is cursor-based: a `marker` query param round-trips with the
//     envelope's <NextMarker> element.
//
// Two host styles are supported, chosen by the optional `endpoint` input:
// the public-cloud default `https://{account}.blob.core.windows.net` (account
// in the host) and the Azurite/path style `http://host:10000/{account}`
// (account as the leading path segment). The canonicalized resource is
// "/{account}{logical path}" in BOTH cases, so BaseURL absorbs the difference
// and the signature code never branches on it.
package storage

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	azure "flomation.app/automate/executor/actions/azure"
)

const (
	// APIVersion is the Blob service version pinned on every request
	// (x-ms-version) and stamped into generated SAS tokens (sv). One constant
	// so a version bump changes signing and requests together.
	APIVersion = "2023-11-03"

	// EntraScope is the client-credentials scope for the Blob service.
	EntraScope = "https://storage.azure.com/.default"

	// maxResponseBody caps ordinary API response bodies (lists, tags, errors).
	maxResponseBody = 8 << 20 // 8 MB

	// MaxDownloadBody caps a blob download. Blobs can be huge; 256 MB is the
	// same ceiling the synchronous copy-from-URL API imposes server-side.
	MaxDownloadBody = 256 << 20 // 256 MB

	// requestTimeout is the HTTP client timeout for a single Blob service call.
	// Generous because a download/upload of maxDownloadBody must fit inside it.
	requestTimeout = 60 * time.Second

	// DefaultPageLimit / MaxPageLimit bound the `maxresults` query param.
	DefaultPageLimit = 50
	MaxPageLimit     = 5000

	// maxListPages bounds a return_all marker walk so a container with tens of
	// millions of blobs can never spin unbounded requests. At 5000 items per
	// page this still admits a million entries.
	maxListPages = 200
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
		key, err := base64.StdEncoding.DecodeString(rawKey)
		if err != nil {
			return Auth{}, fmt.Errorf("account_key is not valid base64 — paste the key exactly as shown under Access keys")
		}
		a.AccountKey = key
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

func clientFor(a Auth) *http.Client {
	if a.Insecure {
		return insecureHTTPClient
	}
	return httpClient
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

// canonicalizedHeaders renders every x-ms-* request header as
// "lowercase-name:value\n", sorted byte-wise by name. The official algorithm
// sorts with .NET en-US culture collation, which differs from a byte sort for
// some hyphenated names — but every header this package emits is an ASCII
// lowercase x-ms-* name, for which the two orders coincide, so the exotic
// sort-key tables the Azure SDKs carry are not needed here.
func canonicalizedHeaders(h http.Header) string {
	names := make([]string, 0, len(h))
	for k := range h {
		if lk := strings.ToLower(k); strings.HasPrefix(lk, "x-ms-") {
			names = append(names, lk)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		b.WriteString(n)
		b.WriteString(":")
		b.WriteString(strings.TrimSpace(h.Get(n)))
		b.WriteString("\n")
	}
	return b.String()
}

// canonicalizedResource is "/{account}{escaped request path}", followed by
// each query parameter as "\nlowercase-key:value" with keys sorted and values
// DECODED (multi-values comma-joined). The path stays in its escaped form —
// that is what the service canonicalizes against.
//
// path MUST be the path of the URL actually sent, not the logical
// container/blob path, and the two differ per host style:
//
//	https://acct.blob.core.windows.net/c/b  -> "/acct" + "/c/b"
//	http://127.0.0.1:10000/acct/c/b         -> "/acct" + "/acct/c/b"
//
// The emulator carries the account in the URL path, so it legitimately
// appears TWICE in the canonicalized resource. Signing the logical path
// instead costs a flat 403 from Azurite — verified against a live emulator,
// and invisible to httptest-based tests, which accept any signature.
func canonicalizedResource(account, path string, q url.Values) string {
	var b strings.Builder
	b.WriteString("/")
	b.WriteString(account)
	if path == "" {
		path = "/"
	}
	b.WriteString(path)
	// Keys sign lowercased but must still look up the original-cased entry.
	lowered := make(map[string][]string, len(q))
	keys := make([]string, 0, len(q))
	for k, vs := range q {
		lk := strings.ToLower(k)
		if _, seen := lowered[lk]; !seen {
			keys = append(keys, lk)
		}
		lowered[lk] = append(lowered[lk], vs...)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString("\n")
		b.WriteString(k)
		b.WriteString(":")
		b.WriteString(strings.Join(lowered[k], ","))
	}
	return b.String()
}

// sharedKeyStringToSign assembles the Shared Key string-to-sign in the
// OFFICIAL slot order — Content-Encoding before Content-Language. (n8n has
// those two swapped; it survives only because it never sets either.) The
// Content-Length slot is empty when the body is zero-length (the
// post-2015-02-21 rule), and the Date slot is always empty because every
// request carries x-ms-date, which takes precedence.
func sharedKeyStringToSign(method string, h http.Header, contentLength int64, account, path string, q url.Values) string {
	lengthStr := ""
	if contentLength > 0 {
		lengthStr = strconv.FormatInt(contentLength, 10)
	}
	return strings.Join([]string{
		method,
		h.Get("Content-Encoding"),
		h.Get("Content-Language"),
		lengthStr,
		h.Get("Content-MD5"),
		h.Get("Content-Type"),
		"", // Date — empty because x-ms-date is always set
		h.Get("If-Modified-Since"),
		h.Get("If-Match"),
		h.Get("If-None-Match"),
		h.Get("If-Unmodified-Since"),
		h.Get("Range"),
	}, "\n") + "\n" + canonicalizedHeaders(h) + canonicalizedResource(account, path, q)
}

// hmacSHA256B64 is the signature primitive shared by request signing and SAS
// generation: base64(HMAC-SHA256(key, message)).
func hmacSHA256B64(key []byte, message string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// APIResponse wraps the HTTP response. Headers is carried because single
// resources return their properties in response headers, not a body.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// Request describes one Blob service call. Path is the LOGICAL resource path
// ("/" for account ops, "/container", "/container/blob"), already
// segment-escaped via ContainerPath/BlobPath — it is appended to BaseURL and
// canonicalized into the signature verbatim.
type Request struct {
	Method  string
	Path    string
	Query   url.Values
	Headers map[string]string
	Body    []byte
	// ContentType is split out of Headers because it participates in the
	// string-to-sign via its own slot, not the x-ms-* canonicalization.
	ContentType string
	// MaxBody overrides the default response cap (blob downloads only).
	MaxBody int64
}

// Do executes one signed Blob service call. Transport and token-mint errors
// come back redacted; callers wrap them in ErrorResult.
func Do(flow *core.Flow, a Auth, r Request) (*APIResponse, error) {
	fullURL := a.BaseURL + r.Path
	if enc := r.Query.Encode(); enc != "" {
		fullURL += "?" + enc
	}

	// Always a *bytes.Reader, even when empty: Go then sends Content-Length: 0
	// on bodyless PUT/POST, which several Blob operations (Put Blob From URL,
	// Snapshot Blob, Set Metadata) require.
	req, err := http.NewRequestWithContext(flow.GoContext(), r.Method, fullURL, bytes.NewReader(r.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %s", a.redact(err.Error()))
	}

	req.Header.Set("x-ms-version", APIVersion)
	req.Header.Set("x-ms-date", nowFunc().UTC().Format(http.TimeFormat))
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	if r.ContentType != "" {
		req.Header.Set("Content-Type", r.ContentType)
	}

	switch a.Method {
	case AuthEntra:
		token, err := azure.ClientCredentialsToken(flow.GoContext(), a.TenantID, a.ClientID, a.ClientSecret, EntraScope)
		if err != nil {
			return nil, err // already redacted by the shared minting code
		}
		req.Header.Set("Authorization", "Bearer "+token)
	default:
		// Sign req.URL's path, not r.Path: a custom endpoint may carry the
		// account (Azurite) or a path prefix of its own, and the service
		// canonicalizes against the path it actually received.
		sts := sharedKeyStringToSign(r.Method, req.Header, int64(len(r.Body)), a.AccountName, req.URL.EscapedPath(), r.Query)
		req.Header.Set("Authorization", "SharedKey "+a.AccountName+":"+hmacSHA256B64(a.AccountKey, sts))
	}

	resp, err := clientFor(a).Do(req)
	if err != nil {
		return nil, fmt.Errorf("Azure Storage request failed: %s", a.redact(err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	maxBody := r.MaxBody
	isDownload := maxBody > 0
	if !isDownload {
		maxBody = maxResponseBody
	}
	// Read ONE byte past the cap so an oversized body is detected instead of
	// silently clipped: a plain LimitReader(maxBody) hits EOF exactly at the
	// cap, which ReadAll reports as success — the caller would then write a
	// truncated prefix of the blob and call it a download.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %s", a.redact(err.Error()))
	}
	if int64(len(body)) > maxBody {
		return nil, overCapError(isDownload, maxBody)
	}
	return &APIResponse{StatusCode: resp.StatusCode, Body: body, Headers: resp.Header}, nil
}

// overCapError names the cap that was hit and the way out of it. The two caps
// have different remedies: a download is fetched in pieces (or handed to the
// service to move), while an envelope over 8 MB means the page asked for too
// much.
func overCapError(isDownload bool, maxBody int64) error {
	if isDownload {
		return fmt.Errorf("the blob is larger than the %d MB download limit — fetch it in pieces with the Byte Range input (e.g. bytes=0-%d), or use Copy Blob, which transfers server-side at any size",
			maxBody>>20, maxBody-1)
	}
	return fmt.Errorf("the Azure Storage response is larger than the %d MB limit — narrow the request (a smaller Limit, or a Prefix)", maxBody>>20)
}

// xmlError is the service's error envelope. AuthenticationErrorDetail carries
// the server's view of the string-to-sign on signature mismatches — priceless
// when debugging, and safe to surface (it never contains the key).
type xmlError struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

// ErrorCode extracts the service error code from a failed response — the XML
// body when there is one, else the x-ms-error-code header (HEAD responses
// carry no body). Empty for 2xx.
func ErrorCode(resp *APIResponse) string {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return ""
	}
	var e xmlError
	if err := xml.Unmarshal(resp.Body, &e); err == nil && e.Code != "" {
		return e.Code
	}
	return resp.Headers.Get("x-ms-error-code")
}

// CheckResponse verifies a 2xx status, decoding the XML <Error> envelope. The
// message keeps only its first line — the service appends RequestId/Time lines
// that are noise in a flow's error output.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var e xmlError
	if err := xml.Unmarshal(resp.Body, &e); err == nil && e.Code != "" {
		msg := strings.SplitN(strings.TrimSpace(e.Message), "\n", 2)[0]
		return fmt.Errorf("Azure Storage error (%d): %s: %s", resp.StatusCode, e.Code, msg)
	}
	if code := resp.Headers.Get("x-ms-error-code"); code != "" {
		return fmt.Errorf("Azure Storage error (%d): %s", resp.StatusCode, code)
	}
	body := string(resp.Body)
	if len(body) > 500 {
		body = body[:500]
	}
	return fmt.Errorf("Azure Storage error (%d): %s", resp.StatusCode, redact(body))
}

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

// MetadataHeaders maps a metadata object input onto x-ms-meta-* headers.
func MetadataHeaders(h map[string]string, inputs []*core.Connection, inputName string) error {
	meta, err := StringMapInput(inputName, inputs)
	if err != nil {
		return err
	}
	for k, v := range meta {
		if !metadataNameRe.MatchString(k) {
			return fmt.Errorf("metadata name %q is invalid: letters, digits and underscores only, not starting with a digit", k)
		}
		h["x-ms-meta-"+k] = v
	}
	return nil
}

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

// TagsHeaderValue renders tags as the query-string form the x-ms-tags upload
// header takes ("k1=v1&k2=v2", both sides URL-encoded), keys sorted for a
// deterministic signature.
func TagsHeaderValue(tags map[string]string) string {
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = url.QueryEscape(k) + "=" + url.QueryEscape(tags[k])
	}
	return strings.Join(parts, "&")
}

// ---------------------------------------------------------------------------
// XML envelopes
// ---------------------------------------------------------------------------

// xmlProps decodes a flat XML element (<Properties>) into a map, camelCasing
// hyphenated element names (Content-Length → contentLength) and coercing
// booleans and integers so a flow can branch on them directly.
//
// It is bound to <Properties> ONLY. Metadata is operator-defined, not a fixed
// schema, so neither transform may touch it — see xmlMeta.
type xmlProps map[string]interface{}

func (m *xmlProps) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	out := map[string]interface{}{}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var s string
			if err := d.DecodeElement(&s, &t); err != nil {
				return err
			}
			out[camelKey(t.Name.Local)] = coerceScalar(s)
		case xml.EndElement:
			if t.Name == start.Name {
				*m = out
				return nil
			}
		}
	}
}

// xmlMeta decodes a <Metadata> element into the SAME shape the header path
// produces (HeadersResult): names lowercased, values kept as verbatim strings.
//
// Both transforms xmlProps applies would corrupt operator-defined metadata —
// camelKey("ORDER_ID") is "oRDER_ID", and coerceScalar("00123") is int64(123)
// with the leading zeros gone — so a flow filtering metadata off a list would
// match nothing that blob_get_properties shows it.
//
// Lowercasing rather than preserving the element name is deliberate: metadata
// travels as x-ms-meta-{name} headers, which are case-INSENSITIVE, so the
// header path cannot recover the case an operator typed and can only ever emit
// a lowercase name. Lowercasing here is what makes one flow expression work
// against both paths.
type xmlMeta map[string]interface{}

func (m *xmlMeta) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	out := map[string]interface{}{}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var s string
			if err := d.DecodeElement(&s, &t); err != nil {
				return err
			}
			out[strings.ToLower(t.Name.Local)] = s
		case xml.EndElement:
			if t.Name == start.Name {
				*m = out
				return nil
			}
		}
	}
}

// camelKey converts a header/element name (Content-Length, x-ms-blob-type
// remainder, BlobType) to lowerCamelCase.
func camelKey(name string) string {
	parts := strings.Split(name, "-")
	var b strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == 0 {
			b.WriteString(strings.ToLower(p[:1]) + p[1:])
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

// coerceScalar turns "true"/"false" into bools and short all-digit strings
// into int64 (content lengths, tag counts). Everything else — etags, dates,
// tier names — stays a string.
func coerceScalar(s string) interface{} {
	switch s {
	case "true":
		return true
	case "false":
		return false
	}
	if len(s) > 0 && len(s) < 19 {
		allDigits := true
		for _, r := range s {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				return n
			}
		}
	}
	return s
}

type xmlTag struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// TagsDocument is the <Tags><TagSet> body of Get/Set Blob Tags.
type TagsDocument struct {
	XMLName xml.Name `xml:"Tags"`
	Tags    []xmlTag `xml:"TagSet>Tag"`
}

// TagsMap flattens a TagSet into a plain object.
func (t TagsDocument) TagsMap() map[string]interface{} {
	out := map[string]interface{}{}
	for _, tag := range t.Tags {
		out[tag.Key] = tag.Value
	}
	return out
}

// TagsXMLBody renders the Set Blob Tags request body, keys sorted so the
// payload (and its Content-MD5-free signature) is deterministic.
func TagsXMLBody(tags map[string]string) []byte {
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	doc := TagsDocument{}
	for _, k := range keys {
		doc.Tags = append(doc.Tags, xmlTag{Key: k, Value: tags[k]})
	}
	body, _ := xml.Marshal(doc)
	return append([]byte(xml.Header), body...)
}

// ListContainer / ListBlob are the per-item shapes inside EnumerationResults.
// One blob struct serves both List Blobs and Find Blobs by Tags — the latter
// adds ContainerName and Tags, the former Properties/Metadata.
type ListContainer struct {
	Name       string   `xml:"Name"`
	Deleted    string   `xml:"Deleted"`
	Version    string   `xml:"Version"`
	Properties xmlProps `xml:"Properties"`
	Metadata   xmlMeta  `xml:"Metadata"`
}

type ListBlob struct {
	Name             string   `xml:"Name"`
	ContainerName    string   `xml:"ContainerName"`
	Snapshot         string   `xml:"Snapshot"`
	VersionID        string   `xml:"VersionId"`
	IsCurrentVersion string   `xml:"IsCurrentVersion"`
	Deleted          string   `xml:"Deleted"`
	Properties       xmlProps `xml:"Properties"`
	Metadata         xmlMeta  `xml:"Metadata"`
	Tags             *struct {
		Tags []xmlTag `xml:"TagSet>Tag"`
	} `xml:"Tags"`
}

// EnumerationResults is the envelope of every list operation. Which item
// slice is populated depends on the operation; NextMarker cursors both.
type EnumerationResults struct {
	XMLName    xml.Name        `xml:"EnumerationResults"`
	Containers []ListContainer `xml:"Containers>Container"`
	Blobs      []ListBlob      `xml:"Blobs>Blob"`
	NextMarker string          `xml:"NextMarker"`
}

// ContainerMap shapes a listed container for output.
func ContainerMap(c ListContainer) map[string]interface{} {
	out := map[string]interface{}{"name": c.Name}
	if len(c.Properties) > 0 {
		out["properties"] = map[string]interface{}(c.Properties)
	}
	if len(c.Metadata) > 0 {
		out["metadata"] = map[string]interface{}(c.Metadata)
	}
	if c.Deleted != "" {
		out["deleted"] = coerceScalar(c.Deleted)
	}
	return out
}

// BlobMap shapes a listed blob for output.
func BlobMap(b ListBlob) map[string]interface{} {
	out := map[string]interface{}{"name": b.Name}
	if b.ContainerName != "" {
		out["container"] = b.ContainerName
	}
	if b.Snapshot != "" {
		out["snapshot"] = b.Snapshot
	}
	if b.VersionID != "" {
		out["versionId"] = b.VersionID
	}
	if b.IsCurrentVersion != "" {
		out["isCurrentVersion"] = coerceScalar(b.IsCurrentVersion)
	}
	if b.Deleted != "" {
		out["deleted"] = coerceScalar(b.Deleted)
	}
	if len(b.Properties) > 0 {
		out["properties"] = map[string]interface{}(b.Properties)
	}
	if len(b.Metadata) > 0 {
		out["metadata"] = map[string]interface{}(b.Metadata)
	}
	if b.Tags != nil {
		tags := map[string]interface{}{}
		for _, t := range b.Tags.Tags {
			tags[t.Key] = t.Value
		}
		out["tags"] = tags
	}
	return out
}

// ListEnumeration fetches one page (or, with returnAll, walks marker ↔
// NextMarker until exhausted) of an EnumerationResults operation. With
// returnAll the page size is raised to the 5000 maximum to minimise round
// trips; otherwise limit is the single page's maxresults. truncated reports
// that the maxListPages backstop stopped the walk early.
func ListEnumeration(flow *core.Flow, a Auth, path string, q url.Values, returnAll bool, limit int) (containers []ListContainer, blobs []ListBlob, truncated bool, err error) {
	pageSize := limit
	if returnAll {
		pageSize = MaxPageLimit
	}
	q.Set("maxresults", strconv.Itoa(pageSize))

	for page := 0; ; page++ {
		if page >= maxListPages {
			return containers, blobs, true, nil
		}
		resp, err := Do(flow, a, Request{Method: http.MethodGet, Path: path, Query: q})
		if err != nil {
			return nil, nil, false, err
		}
		if err := CheckResponse(resp); err != nil {
			return nil, nil, false, err
		}
		var env EnumerationResults
		if err := xml.Unmarshal(resp.Body, &env); err != nil {
			return nil, nil, false, fmt.Errorf("failed to parse list response: %w", err)
		}
		containers = append(containers, env.Containers...)
		blobs = append(blobs, env.Blobs...)
		if !returnAll || env.NextMarker == "" {
			return containers, blobs, false, nil
		}
		q.Set("marker", env.NextMarker)
	}
}

// ---------------------------------------------------------------------------
// Response-header parsing
// ---------------------------------------------------------------------------

// transport headers dropped from properties output — noise for a flow.
var skippedHeaders = map[string]bool{
	"date": true, "server": true, "connection": true, "keep-alive": true,
	"transfer-encoding": true, "accept-ranges": true,
	"x-ms-request-id": true, "x-ms-client-request-id": true, "x-ms-version": true,
}

// HeadersResult shapes a headers-only response (container/blob properties,
// upload confirmations) into {name, properties, metadata}: x-ms-meta-* headers
// become the metadata object, remaining x-ms-* and content headers become
// camelCased properties.
func HeadersResult(name string, h http.Header) map[string]interface{} {
	props := map[string]interface{}{}
	meta := map[string]interface{}{}
	for k, vals := range h {
		if len(vals) == 0 {
			continue
		}
		lk := strings.ToLower(k)
		v := vals[0]
		switch {
		case skippedHeaders[lk]:
		case strings.HasPrefix(lk, "x-ms-meta-"):
			meta[strings.TrimPrefix(lk, "x-ms-meta-")] = v
		case strings.HasPrefix(lk, "x-ms-"):
			props[camelKey(strings.TrimPrefix(lk, "x-ms-"))] = coerceScalar(v)
		case lk == "etag":
			props["etag"] = v
		case strings.HasPrefix(lk, "content-") || lk == "cache-control" || lk == "last-modified":
			props[camelKey(lk)] = coerceScalar(v)
		}
	}
	out := map[string]interface{}{"name": name, "properties": props}
	if len(meta) > 0 {
		out["metadata"] = meta
	}
	return out
}

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

// sasTime is the ISO8601 second-precision UTC format SAS fields use.
func sasTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// sasStringToSign assembles the service-SAS string-to-sign for versions
// 2020-12-06 and later: 16 fields, the last (rsct) without a trailing
// newline. Slots this node doesn't expose (identifier, protocol, snapshot
// time, encryption scope, rscc/rsce/rscl/rsct) sign as empty.
func sasStringToSign(account string, p SASParams) string {
	start := ""
	if !p.Start.IsZero() {
		start = sasTime(p.Start)
	}
	canonical := "/blob/" + account + "/" + p.Container
	if p.Resource == "b" {
		canonical += "/" + p.Blob
	}
	return strings.Join([]string{
		p.Permissions,        // signedPermissions
		start,                // signedStart
		sasTime(p.Expiry),    // signedExpiry
		canonical,            // canonicalizedResource
		"",                   // signedIdentifier
		p.IP,                 // signedIP
		"",                   // signedProtocol
		APIVersion,           // signedVersion
		p.Resource,           // signedResource
		"",                   // signedSnapshotTime
		"",                   // signedEncryptionScope
		"",                   // rscc
		p.ContentDisposition, // rscd
		"",                   // rsce
		"",                   // rscl
		"",                   // rsct
	}, "\n")
}

// BuildServiceSAS signs a service SAS with the account key and returns the
// token (query string, no leading "?"). Shared Key auth only — an Entra
// service principal has no account key to sign with (user-delegation SAS is a
// different, OAuth-bound flow this node does not implement).
func BuildServiceSAS(a Auth, p SASParams) (string, error) {
	if a.Method != AuthSharedKey || len(a.AccountKey) == 0 {
		return "", fmt.Errorf("SAS generation requires Shared Key auth")
	}
	sig := hmacSHA256B64(a.AccountKey, sasStringToSign(a.AccountName, p))

	q := url.Values{}
	q.Set("sv", APIVersion)
	q.Set("sr", p.Resource)
	if !p.Start.IsZero() {
		q.Set("st", sasTime(p.Start))
	}
	q.Set("se", sasTime(p.Expiry))
	q.Set("sp", p.Permissions)
	if p.IP != "" {
		q.Set("sip", p.IP)
	}
	if p.ContentDisposition != "" {
		q.Set("rscd", p.ContentDisposition)
	}
	q.Set("sig", sig)
	return q.Encode(), nil
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

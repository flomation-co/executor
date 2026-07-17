// Package files holds the shared HTTP client, SharedKey/Entra auth, XML
// parsing, and pagination used by every azure/files/* action.
//
// ORIGIN — this file is a deliberate, parameterised COPY of
// actions/azure/storage/common.go (the Blob node), not an import. The two are
// siblings and should be read together:
//
//   - Microsoft documents ONE Shared Key scheme for "Blob, Queue, and File
//     Services" — the same 12-slot string-to-sign, the same canonicalized
//     headers and resource. Verified field-for-field against
//     storage.sharedKeyStringToSign, so the signing code below is a faithful
//     copy rather than a second implementation of a second scheme.
//   - What genuinely differs is Blob-specific hardcoding, and only that: the
//     ".blob." host, the "/blob/" literal in the SAS canonical resource, the
//     blob permission alphabet "racwdxltmei", and an <EnumerationResults>
//     envelope that only knows Containers/Blobs. Each of those is a named
//     constant or a distinct type here.
//
// storage is a different package and azure/storage is out of this change's
// scope, so the shared half could not be extracted without touching a merged,
// live node. Unifying the two behind one signer is a follow-up worth doing —
// when it happens, this header is the map.
//
// Beyond signing, the File service diverges from Blob in three ways that shape
// the code below:
//
//   - There is no single-call upload. Create File allocates a SPARSE file of a
//     declared size and writes NO bytes; Put Range writes them, 4 MiB at a
//     time. See file_upload — a Create with no Put Range leaves a correctly
//     sized, zero-filled file, which looks exactly like success.
//   - Directories are REAL, not a prefix convention. A directory listing
//     returns <Entries> holding both <File> and <Directory>, and a directory
//     must be created before its children and must be empty before it is
//     deleted.
//   - OAuth requires x-ms-file-request-intent: backup on every request, which
//     bypasses the share's file/directory ACLs. See fileRequestIntent.
//
// Two host styles are supported, chosen by the optional `endpoint` input: the
// public-cloud default `https://{account}.file.core.windows.net` (account in
// the host) and a path style `http://host:port/{account}`. The canonicalized
// resource is "/{account}{logical path}" in BOTH cases, so BaseURL absorbs the
// difference and the signature code never branches on it. (Azurite implements
// no File service at all, so the path style exists for sovereign/proxy hosts,
// not for an emulator.)
package files

import (
	"bytes"
	"context"
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
	// APIVersion is the File service version pinned on every request
	// (x-ms-version) and stamped into generated SAS tokens (sv). One constant
	// so a version bump changes signing and requests together.
	//
	// The File service accepts Shared Key from 2014-02-14 onward, but the floor
	// that matters here is higher: x-ms-file-request-intent (OAuth) and
	// x-ms-allow-trailing-dot both need 2022-11-02+. Share stats, share access
	// tiers, leases and ranges are all long-settled below that. 2023-11-03 —
	// the same version azure/storage pins — satisfies every operation used.
	APIVersion = "2023-11-03"

	// EntraScope is the client-credentials scope for the File service. It is
	// the storage-wide scope: Blob, Queue and File share it.
	EntraScope = "https://storage.azure.com/.default"

	// maxResponseBody caps ordinary API response bodies (lists, stats, errors).
	maxResponseBody = 8 << 20 // 8 MB

	// MaxDownloadBody caps a file download, and MaxUploadBody the content a
	// single file_upload will chunk through Put Range. Same ceiling both ways
	// so a download → upload round trip inside one flow cannot fail on the
	// return leg.
	MaxDownloadBody = 256 << 20 // 256 MB
	MaxUploadBody   = 256 << 20 // 256 MB

	// MaxRangeBytes is the hard service cap on ONE Put Range call: "If you
	// attempt to upload a range that's larger than 4 MiB, the service returns
	// status code 413". Content above it is not an error — it is a loop.
	MaxRangeBytes = 4 << 20 // 4 MiB

	// requestTimeout is the HTTP client timeout for a single File service call.
	// Generous because one MaxDownloadBody read must fit inside it.
	requestTimeout = 60 * time.Second

	// DefaultPageLimit / MaxPageLimit bound the `maxresults` query param.
	DefaultPageLimit = 50
	MaxPageLimit     = 5000

	// maxListPages bounds a return_all marker walk so a share with millions of
	// files can never spin unbounded requests. At 5000 entries per page this
	// still admits a million of them.
	maxListPages = 200
)

// nowFunc is the clock for x-ms-date and SAS defaults; a var so the signing
// tests can pin it and assert an exact Authorization header.
var nowFunc = time.Now

// httpClient is shared across every Files action so TLS connections to the
// account endpoint are pooled and reused rather than re-dialled per call.
// insecureHTTPClient is the same but skips TLS verification, used only when
// the action opts in via allow_insecure — a separate client so the secure
// default can never be weakened by a per-request tweak.
//
// DisableCompression is the sharp one, and it carries over from Blob verbatim.
// Content-Encoding is a STORED property of a file, not a transfer encoding:
// Azure serves the bytes exactly as they were written and never compresses on
// the fly. Left enabled, net/http adds its own Accept-Encoding: gzip, reads the
// stored "Content-Encoding: gzip" as its own doing, and hands back the
// DECOMPRESSED body with Content-Encoding and Content-Length stripped — so a
// download of a gzip-encoded file returns different bytes than were uploaded,
// and disagrees with the Range path (net/http skips its gzip handling whenever
// a Range header is present).
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
// these eight first, in this order — files_inputs_drift_test.go enforces it.
//
// The block mirrors azure/storage's field-for-field so an operator moving
// between the Blob and Files nodes sees the same shape. Only two placeholders
// differ, and both had to: the endpoint example names the .file. host, and the
// Entra secret names the FILE data role plus the ACL-bypass consequence that
// choosing Entra on this service carries (see fileRequestIntent).
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
		Placeholder: "The app needs a Storage File Data SMB/Privileged role. Azure requires backup intent on OAuth calls, which BYPASSES the share's file permissions — use Shared Key if the ACLs must apply",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthEntra}},
	},
	{
		Name:        "endpoint",
		Type:        core.ConnectionTypeString,
		Label:       "Custom Endpoint",
		Placeholder: "https://myaccount.file.core.windows.net — leave blank to derive; sovereign clouds only (Azurite has no File service)",
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
// for path-style endpoints], no trailing slash).
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
// 3-24 chars). Enforced because the name is interpolated into a host and into
// the canonicalized resource.
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
		// The one line that makes this the File node rather than the Blob one.
		a.BaseURL = "https://" + account + ".file.core.windows.net"
	} else {
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Auth{}, fmt.Errorf("endpoint must be an http(s) URL, e.g. https://myaccount.file.core.windows.net")
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

// acquireToken indirects the Entra token exchange so tests can stub it; the
// real path is the shared per-execution cache in actions/azure/common.go.
var acquireToken = func(ctx context.Context, a Auth) (string, error) {
	return azure.ClientCredentialsToken(ctx, a.TenantID, a.ClientID, a.ClientSecret, EntraScope)
}

// SetTokenForTest bypasses the real Entra token exchange, handing every
// request the given bearer token, and returns a restore function. Test-only.
func SetTokenForTest(token string) func() {
	prev := acquireToken
	acquireToken = func(context.Context, Auth) (string, error) { return token, nil }
	return func() { acquireToken = prev }
}

// ---------------------------------------------------------------------------
// Redaction
// ---------------------------------------------------------------------------

// sasSigRe matches a SAS signature query value so a URL echoed into an error
// (e.g. by a transport failure on a copy source) never leaks it.
var sasSigRe = regexp.MustCompile(`(sig=)[^&\s"']+`)

// redact scrubs credential material from an error string: SAS signatures in
// URLs. Auth-aware masking of the literal key and client secret is layered on
// top by Auth.redact.
func redact(msg string) string {
	return sasSigRe.ReplaceAllString(msg, "${1}REDACTED")
}

// RedactURL scrubs the SAS signature out of a URL that is about to be echoed
// into an action's OUTPUT, where errors are not the only leak: result objects
// are persisted in the run record and forwarded to every downstream node. Only
// sig= is a credential — the rest of a SAS (sv/sp/se) identifies WHICH source
// was read, which is provenance worth keeping.
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
// share/file path, and the two differ per host style:
//
//	https://acct.file.core.windows.net/s/f  -> "/acct" + "/s/f"
//	http://host:port/acct/s/f               -> "/acct" + "/acct/s/f"
//
// A path-style endpoint carries the account in the URL path, so it legitimately
// appears TWICE in the canonicalized resource. This is inherited from the Blob
// node, where signing the logical path instead cost a flat 403 from a live
// emulator — invisible to httptest-based tests, which accept any signature.
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
// OFFICIAL slot order — Content-Encoding before Content-Language. The
// Content-Length slot is empty when the body is zero-length (the
// post-2015-02-21 rule), and the Date slot is always empty because every
// request carries x-ms-date, which takes precedence.
//
// Microsoft publishes this as ONE scheme for Blob, Queue and File, and this is
// the same assembly as storage.sharedKeyStringToSign — the Table service is
// the outlier that would need its own (no canonicalized headers, Date never
// empty), and it is not this node's problem.
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

// fileRequestIntent is the value of x-ms-file-request-intent, which the File
// service REQUIRES on every OAuth-authorized request and rejects the absence of
// with a bare 400. "backup" is the only value it accepts.
//
// It is not a formality. The header requests read/writeFileBackupSemantics,
// which BYPASSES the share's file and directory permissions — so choosing
// Microsoft Entra on this node means "ignore the ACLs", where on the Blob node
// it means only "use RBAC instead of the key". Shared Key remains the default
// for exactly that reason, and the client-secret placeholder says so out loud.
const fileRequestIntent = "backup"

// APIResponse wraps the HTTP response. Headers is carried because single
// resources return their properties in response headers, not a body.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// Request describes one File service call. Path is the LOGICAL resource path
// ("/" for account ops, "/share", "/share/dir/file"), already segment-escaped
// via SharePath/DirectoryPath/FilePath — it is appended to BaseURL and
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
	// MaxBody overrides the default response cap (file downloads only).
	MaxBody int64
}

// Do executes one signed File service call. Transport and token-mint errors
// come back redacted; callers wrap them in ErrorResult.
func Do(flow *core.Flow, a Auth, r Request) (*APIResponse, error) {
	fullURL := a.BaseURL + r.Path
	if enc := r.Query.Encode(); enc != "" {
		fullURL += "?" + enc
	}

	// Always a *bytes.Reader, even when empty: Go then sends Content-Length: 0
	// on bodyless PUT/POST, which Create File, Set Metadata and the lease
	// operations require.
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
	applyTrailingDot(req.Header, r)

	switch a.Method {
	case AuthEntra:
		token, err := acquireToken(flow.GoContext(), a)
		if err != nil {
			return nil, err // already redacted by the shared minting code
		}
		req.Header.Set("Authorization", "Bearer "+token)
		// Without this the service answers 400 on every OAuth call. Set after
		// the caller's headers so an action cannot accidentally unset it, and
		// before signing so it is not part of a Shared Key signature it has no
		// business in (Shared Key never reaches this branch).
		req.Header.Set("x-ms-file-request-intent", fileRequestIntent)
	default:
		// Sign req.URL's path, not r.Path: a custom endpoint may carry the
		// account or a path prefix of its own, and the service canonicalizes
		// against the path it actually received.
		sts := sharedKeyStringToSign(r.Method, req.Header, int64(len(r.Body)), a.AccountName, req.URL.EscapedPath(), r.Query)
		req.Header.Set("Authorization", "SharedKey "+a.AccountName+":"+hmacSHA256B64(a.AccountKey, sts))
	}

	resp, err := clientFor(a).Do(req)
	if err != nil {
		return nil, fmt.Errorf("Azure Files request failed: %s", a.redact(err.Error()))
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
	// truncated prefix of the file and call it a download.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %s", a.redact(err.Error()))
	}
	if int64(len(body)) > maxBody {
		return nil, overCapError(isDownload, maxBody)
	}
	return &APIResponse{StatusCode: resp.StatusCode, Body: body, Headers: resp.Header}, nil
}

// applyTrailingDot opts this node into verbatim path names. Without
// x-ms-allow-trailing-dot the service SILENTLY TRIMS a trailing dot from a file
// or directory name, so "report." is created as "report" and every later call
// naming "report." 404s. The header only applies to paths BELOW the share (a
// share name cannot end in a dot at all), so it is sent only when the path has
// a segment past the share — and its source twin is sent alongside whenever the
// request names a copy source, which is subject to the same trimming.
func applyTrailingDot(h http.Header, r Request) {
	if strings.Count(strings.Trim(r.Path, "/"), "/") == 0 {
		return // account root or a share-level operation
	}
	h.Set("x-ms-allow-trailing-dot", "true")
	if h.Get("x-ms-copy-source") != "" {
		h.Set("x-ms-source-allow-trailing-dot", "true")
	}
}

// overCapError names the cap that was hit and the way out of it. The two caps
// have different remedies: a download is fetched in pieces (or handed to the
// service to move), while an envelope over 8 MB means the page asked for too
// much.
func overCapError(isDownload bool, maxBody int64) error {
	if isDownload {
		return fmt.Errorf("the file is larger than the %d MB download limit — fetch it in pieces with the Byte Range input (e.g. bytes=0-%d), or use Copy File, which transfers server-side at any size",
			maxBody>>20, maxBody-1)
	}
	return fmt.Errorf("the Azure Files response is larger than the %d MB limit — narrow the request (a smaller Limit, or a Prefix)", maxBody>>20)
}

// xmlError is the service's error envelope.
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
		return fmt.Errorf("Azure Files error (%d): %s: %s", resp.StatusCode, e.Code, msg)
	}
	if code := resp.Headers.Get("x-ms-error-code"); code != "" {
		return fmt.Errorf("Azure Files error (%d): %s", resp.StatusCode, code)
	}
	body := string(resp.Body)
	if len(body) > 500 {
		body = body[:500]
	}
	return fmt.Errorf("Azure Files error (%d): %s", resp.StatusCode, redact(body))
}

// ---------------------------------------------------------------------------
// Paths & names
// ---------------------------------------------------------------------------

// shareNameRe: lowercase letters/digits/hyphens, starting and ending
// alphanumeric. Length (3-63) and consecutive hyphens are checked separately
// (the class can't express either cleanly). Identical to the container rule —
// shares and containers are named by the same DNS-label constraint.
var shareNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// ValidateShareName enforces the service's share naming rules client-side so
// the operator gets a readable message instead of a signed request that 400s.
func ValidateShareName(name string) error {
	if len(name) < 3 || len(name) > 63 || !shareNameRe.MatchString(name) || strings.Contains(name, "--") {
		return fmt.Errorf("share name %q is invalid: 3-63 lowercase letters, digits and hyphens, starting and ending with a letter or digit, no consecutive hyphens", name)
	}
	return nil
}

// pathIllegalRe is the reserved set for a file or directory NAME. File and
// directory names are nothing like share names: they are CASE-PRESERVING and
// accept a wide charset, so the share rule must not be reused on them (doing so
// would reject "Reports 2026/Q1 Summary.pdf", which is perfectly legal). Only
// the SMB-reserved characters are out — and "/" is absent from the class on
// purpose, because it is the separator the caller splits on before validating
// each segment.
var pathIllegalRe = regexp.MustCompile(`["\\:|<>*?]`)

// ValidateFilePath enforces the file/directory naming rules on a slash-separated
// path: 1-255 characters per segment, no reserved characters, no empty segment.
// A trailing dot is NOT rejected — the service accepts it and this node opts
// into keeping it (see applyTrailingDot) rather than having it silently trimmed.
func ValidateFilePath(field, p string) error {
	if p == "" {
		return fmt.Errorf("%s is required", field)
	}
	if len(p) > 1024 {
		return fmt.Errorf("%s is longer than the 1024-character path limit", field)
	}
	for _, seg := range strings.Split(p, "/") {
		switch {
		case seg == "":
			return fmt.Errorf("%s %q has an empty path segment — use single slashes, with no leading or trailing slash", field, p)
		case len(seg) > 255:
			return fmt.Errorf("%s %q has a segment longer than 255 characters", field, seg)
		case pathIllegalRe.MatchString(seg):
			return fmt.Errorf(`%s %q contains a reserved character — " \ : | < > * ? are not allowed in a file or directory name`, field, seg)
		}
	}
	return nil
}

// escapePath segment-escapes a slash-separated path, leaving the separators
// intact. Names may contain spaces and "#" (the service allows both), which
// interpolated raw would break the request and the signature alike.
func escapePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// SharePath returns the escaped logical path for a share.
func SharePath(share string) string {
	return "/" + url.PathEscape(share)
}

// DirectoryPath returns the escaped logical path for a directory inside a
// share. An empty dir is the share's ROOT directory, which the service
// addresses as the share itself — the caller still sends restype=directory, so
// the two are not confusable on the wire.
func DirectoryPath(share, dir string) string {
	dir = strings.Trim(dir, "/")
	if dir == "" {
		return SharePath(share)
	}
	return SharePath(share) + "/" + escapePath(dir)
}

// FilePath returns the escaped logical path for a file, with an optional
// directory prefix.
func FilePath(share, dir, file string) string {
	return DirectoryPath(share, dir) + "/" + escapePath(file)
}

// JoinPath renders the human-readable (unescaped) path of a file for summaries
// and outputs.
func JoinPath(dir, file string) string {
	dir = strings.Trim(dir, "/")
	if dir == "" {
		return file
	}
	return dir + "/" + file
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

// StringMapInput parses an object input (metadata) into a flat string→string
// map, coercing scalar values to strings. Returns (nil, nil) when absent.
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

// ShareIncludeTokens is the full set the List Shares `include` query param
// accepts. Anything outside it is a 400 InvalidQueryParameterValue that names
// nothing useful, so the tokens are checked here instead.
var ShareIncludeTokens = []string{"metadata", "snapshots", "deleted"}

// ParseIncludeTokens turns an `include` input into the comma-separated value
// the query param takes. The input is a ComboBox — the Options shortlist covers
// the common single choices, and free text is what allows them to be COMBINED
// ("metadata,snapshots"), which is the only way to get both in one listing pass.
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

// ---------------------------------------------------------------------------
// Leases
// ---------------------------------------------------------------------------

// A lease is a write lock on a file, held for a fixed duration (or
// indefinitely) and identified by a GUID. Two halves live here:
//
//   - LeaseIDInput / LeaseHeader — the x-ms-lease-id an EXISTING action sends
//     to prove it holds the lock. Without it, every write to a leased file is
//     refused with 412 LeaseIdMissing. Reads are not blocked by a lease, but
//     the header is still accepted on them as an assertion: "fail unless the
//     lease is active and mine".
//   - LeaseActionOptions / BuildLeaseCall / LeaseResult — the lifecycle
//     (PUT ?comp=lease) that MINTS those IDs.
//
// The File service's lease durations differ from Blob's: a FILE lease is
// infinite-only (-1). The 15-60s finite window Blob offers exists on Files only
// for SHARE leases, which this node does not expose — so an acquire here always
// signs -1, and the duration input Blob carries has no counterpart.
const (
	LeaseAcquire = "acquire"
	LeaseChange  = "change"
	LeaseRelease = "release"
	LeaseBreak   = "break"
)

// LeaseInfiniteDuration is the only duration a FILE lease accepts. It is the
// sharp one: the lease survives the flow that took it, so a flow that acquires
// and never releases locks the file until somebody breaks it.
const LeaseInfiniteDuration = -1

// LeaseActionOptions is the lease_action dropdown. Renew is absent on purpose —
// an infinite file lease has nothing to renew, and the service rejects the
// action on a file.
var LeaseActionOptions = []core.ConnectionOption{
	{Name: "Acquire — take the lock", Value: LeaseAcquire},
	{Name: "Change — swap the lock's ID", Value: LeaseChange},
	{Name: "Release — hand the lock back", Value: LeaseRelease},
	{Name: "Break — end someone else's lock", Value: LeaseBreak},
}

// LeaseIDInput is the canonical optional lease-id field carried by every action
// that touches a file somebody may have leased. Like AuthInputs it is
// documentation rather than enforcement (the manifest generator AST-parses each
// action's literal), so files_inputs_drift_test.go compares the copies against
// it.
//
// It is deliberately NOT part of the credential block: a lease ID is an
// operator-supplied fact about one call, not a credential, so it sits with the
// resource fields and the auth-block drift assertion stays untouched.
var LeaseIDInput = core.Connection{
	Name:        "lease_id",
	Type:        core.ConnectionTypeString,
	Label:       "Lease ID",
	Placeholder: "Only needed when the file is leased — the Lease ID output of a Lease File step",
}

// LeaseHeader adds x-ms-lease-id to h when the action's lease_id input is set,
// allocating h when the caller had no headers of its own. A BLANK input must
// send no header at all: an empty x-ms-lease-id is not "no lease", it is an
// invalid one, and the service answers 400 rather than performing the unleased
// operation the operator meant.
func LeaseHeader(h map[string]string, inputs []*core.Connection) map[string]string {
	id := OptionalString("lease_id", inputs)
	if id == "" {
		return h
	}
	if h == nil {
		h = map[string]string{}
	}
	h["x-ms-lease-id"] = id
	return h
}

// leaseIDRe is the GUID form Azure requires of a proposed lease ID. The service
// rejects anything else with a bare 400 InvalidHeaderValue naming only the
// header, so the check is worth making here where the message can name the
// field and the rule.
var leaseIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// LeaseCall is one resolved Lease File request: the action chosen and the
// x-ms-lease-* headers it implies.
type LeaseCall struct {
	Action  string
	Headers map[string]string
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

	c := LeaseCall{Action: action, Headers: map[string]string{"x-ms-lease-action": action}}
	leaseID := OptionalString("lease_id", inputs)
	proposed := OptionalString("proposed_lease_id", inputs)

	switch action {
	case LeaseAcquire:
		// A file lease is infinite or nothing — the service rejects any other
		// duration on a file, so there is no input to read here.
		c.Headers["x-ms-lease-duration"] = strconv.Itoa(LeaseInfiniteDuration)
		// On acquire the ID travels as x-ms-proposed-lease-id, never
		// x-ms-lease-id: there is no lease yet to name. Blank means the service
		// mints one and reports it back.
		if proposed != "" {
			if !leaseIDRe.MatchString(proposed) {
				return LeaseCall{}, fmt.Errorf("proposed_lease_id must be a GUID, e.g. 8b1c6a2e-0f9d-4a3b-9c5e-7d2f1a4b6c8d (got %q) — leave it blank to let Azure choose one", proposed)
			}
			c.Headers["x-ms-proposed-lease-id"] = proposed
		}
	case LeaseRelease:
		if leaseID == "" {
			return LeaseCall{}, fmt.Errorf("lease_id is required to release a lease — it is the Lease ID output of the Acquire step")
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
	default:
		return LeaseCall{}, fmt.Errorf("lease_action %q is not supported (use acquire, change, release or break)", action)
	}
	return c, nil
}

// LeaseResult shapes a lease response into the standard action output plus the
// thing a downstream node needs: lease_id — the whole point, since every leased
// write must quote it. target is how the file is named in the summary.
func LeaseResult(c LeaseCall, id, target string, resp *APIResponse) map[string]interface{} {
	leaseID := resp.Headers.Get("x-ms-lease-id")

	var summary string
	switch c.Action {
	case LeaseAcquire:
		summary = fmt.Sprintf("Acquired an infinite lease on %s — release it, or the file stays locked", target)
	case LeaseChange:
		summary = fmt.Sprintf("Changed the lease ID on %s", target)
	case LeaseRelease:
		summary = fmt.Sprintf("Released the lease on %s", target)
	case LeaseBreak:
		summary = fmt.Sprintf("Broke the lease on %s", target)
	}

	out := ResourceResult(id, HeadersResult(id, resp.Headers), summary)
	result := out["result"].(map[string]interface{})
	result["leaseAction"] = c.Action
	out["lease_id"] = leaseID
	return out
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
// match nothing that share_get_properties shows it.
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

// camelKey converts a header/element name (Content-Length, BlobType) to
// lowerCamelCase.
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

// coerceScalar turns "true"/"false" into bools and short all-digit strings into
// int64 (content lengths, quotas). Everything else — etags, dates, tier names —
// stays a string.
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

// ListShare is one <Share> inside a List Shares envelope.
type ListShare struct {
	Name       string   `xml:"Name"`
	Snapshot   string   `xml:"Snapshot"`
	Deleted    string   `xml:"Deleted"`
	Version    string   `xml:"Version"`
	Properties xmlProps `xml:"Properties"`
	Metadata   xmlMeta  `xml:"Metadata"`
}

// ListEntry is one <File> or <Directory> inside a directory listing's
// <Entries>. One struct serves both — a directory carries no <Properties>
// worth speaking of, and the entry's KIND is what tells them apart, which is
// why EnumerationResults decodes them into two slices rather than one.
type ListEntry struct {
	Name       string   `xml:"Name"`
	Properties xmlProps `xml:"Properties"`
	Attributes string   `xml:"Attributes"`
}

// EnumerationResults is the envelope of every list operation. Which slice is
// populated depends on the operation; NextMarker cursors all of them.
//
// This is where Files parts company with Blob most visibly. A blob "directory"
// is a naming convention — List Blobs fakes hierarchy with prefixes. Here the
// hierarchy is REAL: one <Entries> element holds <File> and <Directory>
// children, and they are genuinely different things. Decoding them into two
// slices loses their interleaved order, which no consumer needs and no cursor
// depends on.
type EnumerationResults struct {
	XMLName     xml.Name    `xml:"EnumerationResults"`
	Shares      []ListShare `xml:"Shares>Share"`
	Files       []ListEntry `xml:"Entries>File"`
	Directories []ListEntry `xml:"Entries>Directory"`
	NextMarker  string      `xml:"NextMarker"`
}

// ShareStats is the body of Get Share Stats — the ONE share operation that
// answers with XML rather than headers. ShareUsageBytes is the number an
// operator actually wants: the quota is what you set, this is what you used.
type ShareStats struct {
	XMLName         xml.Name `xml:"ShareStats"`
	ShareUsageBytes int64    `xml:"ShareUsageBytes"`
}

// RangeList is the body of List Ranges. Ranges are the written regions of a
// sparse file; ClearRanges appear only when a previous snapshot is compared
// against, and are carried so the output does not silently drop them.
type RangeList struct {
	XMLName     xml.Name    `xml:"Ranges"`
	Ranges      []ByteRange `xml:"Range"`
	ClearRanges []ByteRange `xml:"ClearRange"`
}

// ByteRange is one inclusive [Start, End] byte span.
type ByteRange struct {
	Start int64 `xml:"Start"`
	End   int64 `xml:"End"`
}

// ShareMap shapes a listed share for output.
func ShareMap(s ListShare) map[string]interface{} {
	out := map[string]interface{}{"name": s.Name}
	if s.Snapshot != "" {
		out["snapshot"] = s.Snapshot
	}
	if s.Deleted != "" {
		out["deleted"] = coerceScalar(s.Deleted)
	}
	if s.Version != "" {
		out["version"] = s.Version
	}
	if len(s.Properties) > 0 {
		out["properties"] = map[string]interface{}(s.Properties)
	}
	if len(s.Metadata) > 0 {
		out["metadata"] = map[string]interface{}(s.Metadata)
	}
	return out
}

// EntryMap shapes a listed directory entry for output. The `type` key is the
// point: a flow reading a directory listing has to be able to tell a file from
// a directory, and unlike a blob listing there is no name convention to infer
// it from.
func EntryMap(e ListEntry, kind string) map[string]interface{} {
	out := map[string]interface{}{"name": e.Name, "type": kind}
	if len(e.Properties) > 0 {
		out["properties"] = map[string]interface{}(e.Properties)
	}
	if e.Attributes != "" {
		out["attributes"] = e.Attributes
	}
	return out
}

// ListEnumeration fetches one page (or, with returnAll, walks marker ↔
// NextMarker until exhausted) of an EnumerationResults operation. With
// returnAll the page size is raised to the 5000 maximum to minimise round
// trips; otherwise limit is the single page's maxresults. truncated reports
// that the maxListPages backstop stopped the walk early.
func ListEnumeration(flow *core.Flow, a Auth, path string, q url.Values, returnAll bool, limit int) (shares []ListShare, dirs []ListEntry, fileEntries []ListEntry, truncated bool, err error) {
	pageSize := limit
	if returnAll {
		pageSize = MaxPageLimit
	}
	q.Set("maxresults", strconv.Itoa(pageSize))

	for page := 0; ; page++ {
		if page >= maxListPages {
			return shares, dirs, fileEntries, true, nil
		}
		resp, err := Do(flow, a, Request{Method: http.MethodGet, Path: path, Query: q})
		if err != nil {
			return nil, nil, nil, false, err
		}
		if err := CheckResponse(resp); err != nil {
			return nil, nil, nil, false, err
		}
		var env EnumerationResults
		if err := xml.Unmarshal(resp.Body, &env); err != nil {
			return nil, nil, nil, false, fmt.Errorf("failed to parse list response: %w", err)
		}
		shares = append(shares, env.Shares...)
		dirs = append(dirs, env.Directories...)
		fileEntries = append(fileEntries, env.Files...)
		if !returnAll || env.NextMarker == "" {
			return shares, dirs, fileEntries, false, nil
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

// HeadersResult shapes a headers-only response (share/file properties, write
// confirmations) into {name, properties, metadata}: x-ms-meta-* headers become
// the metadata object, remaining x-ms-* and content headers become camelCased
// properties.
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

// sasServicePrefix is the canonicalized-resource service segment. Blob signs
// "/blob/", File signs "/file/" — the same account, the same key, a different
// literal, and a token signed with the wrong one is rejected with a signature
// mismatch that names neither.
const sasServicePrefix = "/file/"

// sasPermissionOrder is the canonical permission ordering for a FILE service
// SAS: read, create, write, delete, list. It is not blob's alphabet —
// "racwdxltmei" carries permissions (append, tags, immutability, execute) that
// have no meaning on a file share, and signing one would produce a token the
// service refuses.
//
// The service rejects tokens whose sp string is out of order, so we validate
// rather than silently reorder — an operator who typed "wr" should learn the
// rule, not get a token that means something else.
const sasPermissionOrder = "rcwdl"

// SAS resource kinds. "l" (list) is meaningful on a share only; a file SAS with
// it is rejected.
const (
	SASResourceFile  = "f"
	SASResourceShare = "s"
)

// ValidateSASPermissions checks perms is a non-empty subset of
// sasPermissionOrder, in canonical order, without duplicates. resource gates
// the share-only "l".
func ValidateSASPermissions(perms, resource string) error {
	if perms == "" {
		return fmt.Errorf("permissions is required (e.g. \"r\" for read-only)")
	}
	last := -1
	for _, r := range perms {
		idx := strings.IndexRune(sasPermissionOrder, r)
		if idx < 0 {
			return fmt.Errorf("permission %q is not valid: use characters from %q (read, create, write, delete, list)", string(r), sasPermissionOrder)
		}
		if idx <= last {
			return fmt.Errorf("permissions %q are out of order or duplicated: follow the order %q", perms, sasPermissionOrder)
		}
		if r == 'l' && resource != SASResourceShare {
			return fmt.Errorf(`the "l" (list) permission applies to a share, not a single file`)
		}
		last = idx
	}
	return nil
}

// SASParams are the knobs of a service SAS. Share/Path are the RAW (unescaped)
// names — the SAS string-to-sign canonicalizes the DECODED path.
type SASParams struct {
	Resource           string // SASResourceFile or SASResourceShare
	Share              string
	Path               string // dir/file, file resource only
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

// sasStringToSign assembles the FILE service-SAS string-to-sign for versions
// 2020-12-06 and later: THIRTEEN fields, the last (rsct) without a trailing
// newline. Slots this node doesn't expose (identifier, protocol,
// rscc/rsce/rscl/rsct) sign as empty.
//
// This is where the "Blob and File share one scheme" premise STOPS, and the
// distinction is not the canonical resource. Shared Key request signing really
// is one scheme across Blob/Queue/File — but the service SAS is not:
//
//	Blob: 16 fields — …signedVersion, signedResource, signedSnapshotTime,
//	                  signedEncryptionScope, rscc, rscd, rsce, rscl, rsct
//	File: 13 fields — …signedVersion, rscc, rscd, rsce, rscl, rsct
//
// signedResource, signedSnapshotTime and signedEncryptionScope are BLOB-ONLY.
// Signing the Blob layout here put "f" where the service expects rscc and
// pushed rscd three slots down, so every generated link was rejected with a
// bare AuthenticationFailed — verified against a real account, which echoes the
// 13-field string it used back in AuthenticationErrorDetail.
//
// sr is still SENT as a query parameter (the service needs it to know whether
// the token scopes a file or a share); it is simply not signed.
//
// No mock can catch this: an httptest server accepts whatever signature it is
// handed, so the only proof is a real 200 from Azure.
func sasStringToSign(account string, p SASParams) string {
	start := ""
	if !p.Start.IsZero() {
		start = sasTime(p.Start)
	}
	canonical := sasServicePrefix + account + "/" + p.Share
	if p.Resource == SASResourceFile {
		canonical += "/" + strings.Trim(p.Path, "/")
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
		"",                   // rscc
		p.ContentDisposition, // rscd
		"",                   // rsce
		"",                   // rscl
		"",                   // rsct
	}, "\n")
}

// BuildServiceSAS signs a service SAS with the account key and returns the
// token (query string, no leading "?"). Shared Key auth only — an Entra service
// principal has no account key to sign with (user-delegation SAS is a
// different, OAuth-bound flow this node does not implement, and on Files it
// would inherit the backup-intent ACL bypass besides).
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

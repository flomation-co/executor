package core

// BlobStore — API-backed off-load tier for large tool-call outputs.
// Underpins the same contract as the original disk-backed BlobStore
// (per-execution Put/Get/Cleanup, opaque flo:blob:... tokens that
// the AI passes through unchanged) but persistence happens in the
// API's blob_object table, accessed over mTLS by ctx.InternalClient().
//
// Why the change? Two reasons that the disk-backed predecessor
// couldn't satisfy:
//
//  1. Inbound files from Launch (M2+) need to land in the same tier
//     before any execution exists. A local disk that lives in the
//     executor's per-execution cwd cannot be written to by Launch.
//     With the API tier, Launch uploads a blob, the executor reads it
//     back during the resulting flow — same handle, same auth scope.
//  2. Tool-output blobs were dying with their execution. The API
//     tier's TTL (1 h for tool_output, 30 d for inbound) keeps them
//     available for the editor's media inspector and any future
//     downstream consumer without coupling lifetime to the local
//     working directory.
//
// Handles are 16 bytes (32 hex characters), set by the API on
// upload. The executor's previous 8-byte handle format is GONE — any
// in-flight code that depends on 16-character handles will see
// ParseBlobToken fail. There is no migration: the per-execution
// blob_store has no persisted state, and the format change is part
// of unifying the contract across Launch / executor / API.

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// BlobThresholdBytes is the size at which a string output is
// considered worth off-loading. Picked so it's comfortably larger
// than any rendering-meaningful summary (typical "tool_result"
// strings are 100–500 chars) but smaller than any plausible single
// media payload (an MP3 second of audio is ~16 KB base64-encoded).
const BlobThresholdBytes = 10 * 1024

// BlobTokenPrefix marks a string value as a blob reference. Same
// prefix as before — the wire format is intentionally portable
// between the disk and API backends so any in-flight serialised
// data continues to parse.
const BlobTokenPrefix = "flo:blob:"

// OrgIDHeader is the per-request org scope the API uses to enforce
// auth. Mirrors the api package's constant; duplicated here only
// because we'd otherwise import the API module for a single string.
const OrgIDHeader = "X-Flomation-Org-Id"

// OwnerIDHeader is the personal-mode counterpart of OrgIDHeader,
// used for tool-output storage when the executing flow has no
// organisation context. Exactly one of (OrgID, OwnerID) must be set
// per request — the executor's NewBlobStore caller picks based on
// ExecutionContext.OrganisationID falling back to AuthorID.
const OwnerIDHeader = "X-Flomation-Owner-Id"

// blobUploadPurpose tells the API which TTL bucket to apply. The
// executor only ever produces tool-output blobs; inbound is set by
// Launch in M2+, manual is reserved for future admin uploads.
const blobUploadPurpose = "tool_output"

// BlobStore is the per-execution facade over the API's blob endpoints.
// Safe for concurrent use — the cache mutex serialises map access.
//
// orgID and ownerID are mutually exclusive — exactly one is non-empty
// at construction time, mirroring the API's BlobScope discriminated
// union. The lookup helper scopeHeader picks the right request header
// based on which is set.
type BlobStore struct {
	client      *http.Client
	apiURL      string
	orgID       string
	ownerID     string
	executionID string

	mu    sync.Mutex
	cache map[string][]byte // handle (hex) -> bytes; bounded by cacheMaxEntries
}

// cacheMaxEntries caps the in-process Get cache. Lets one execution
// reuse a token across many tool calls without re-fetching, but
// can't grow unboundedly. Eviction is naive: hit the cap, drop the
// whole map. The blobs are still on the server.
const cacheMaxEntries = 32

// NewBlobStore builds a per-execution store bound to the calling
// execution's identity. orgID and ownerID are mutually exclusive —
// pass orgID for organisation-scoped flows, ownerID for personal-mode
// flows. Setting both is a contract violation: Put will refuse the
// upload with a clear error. Setting neither is also a violation.
// apiURL is required; the cache lives inside the struct.
func NewBlobStore(client *http.Client, apiURL, orgID, ownerID, executionID string) *BlobStore {
	return &BlobStore{
		client:      client,
		apiURL:      strings.TrimRight(apiURL, "/"),
		orgID:       orgID,
		ownerID:     ownerID,
		executionID: executionID,
		cache:       map[string][]byte{},
	}
}

// scopeHeader returns the (header, value) pair the API expects on
// each request. Returns ("", "") when the store is misconfigured —
// callers should reject before sending.
func (s *BlobStore) scopeHeader() (header, value string) {
	if s == nil {
		return "", ""
	}
	if s.orgID != "" && s.ownerID == "" {
		return OrgIDHeader, s.orgID
	}
	if s.ownerID != "" && s.orgID == "" {
		return OwnerIDHeader, s.ownerID
	}
	return "", ""
}

// Put uploads value to the API tier and returns a verbose token of
// the form
//
//	flo:blob:<32-hex>?size=<bytes>&type=<mime>
//
// The mimeHint is forwarded as the `mime` form field — the API will
// reject the upload if it disagrees with the sniffed content category.
func (s *BlobStore) Put(value []byte, mimeHint string) (string, error) {
	if s == nil || s.apiURL == "" {
		return "", fmt.Errorf("blob store not initialised: missing apiURL")
	}
	headerKey, headerVal := s.scopeHeader()
	if headerKey == "" {
		return "", fmt.Errorf("blob store not initialised: exactly one of orgID / ownerID required")
	}
	if mimeHint == "" {
		mimeHint = "application/octet-stream"
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("mime", mimeHint); err != nil {
		return "", fmt.Errorf("write mime field: %w", err)
	}
	if err := w.WriteField("purpose", blobUploadPurpose); err != nil {
		return "", fmt.Errorf("write purpose field: %w", err)
	}
	if s.executionID != "" {
		if err := w.WriteField("execution_id", s.executionID); err != nil {
			return "", fmt.Errorf("write execution_id field: %w", err)
		}
	}
	part, err := w.CreateFormFile("file", "blob.bin")
	if err != nil {
		return "", fmt.Errorf("create file part: %w", err)
	}
	if _, err = part.Write(value); err != nil {
		return "", fmt.Errorf("write file bytes: %w", err)
	}
	if err = w.Close(); err != nil {
		return "", fmt.Errorf("close multipart: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, s.apiURL+"/api/v1/internal/blob", &body)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set(headerKey, headerVal)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("blob upload: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("blob upload: status %d: %s", resp.StatusCode, truncateForErr(string(respBody)))
	}

	// Parse the canonical token out of the JSON response. We rely on
	// the API's token rather than reconstructing one client-side —
	// keeps the format authoritative in one place even if it evolves.
	token, err := extractTokenFromUploadResponse(respBody)
	if err != nil {
		return "", err
	}
	return token, nil
}

// Get resolves a token back to the original bytes. Tolerates the
// full verbose token format (with query params) and bare-handle
// shortcuts. Caches successful reads within the execution so a
// downstream tool that passes a token through twice only fetches
// once.
func (s *BlobStore) Get(token string) ([]byte, error) {
	if s == nil || s.apiURL == "" {
		return nil, fmt.Errorf("blob store not initialised: missing apiURL")
	}

	handle, _, _, ok := ParseBlobToken(token)
	if !ok {
		return nil, fmt.Errorf("not a blob token: %q", truncateForErr(token))
	}

	s.mu.Lock()
	if cached, hit := s.cache[handle]; hit {
		s.mu.Unlock()
		out := make([]byte, len(cached))
		copy(out, cached)
		return out, nil
	}
	s.mu.Unlock()

	req, err := http.NewRequest(http.MethodGet,
		s.apiURL+"/api/v1/internal/blob/"+handle, nil)
	if err != nil {
		return nil, fmt.Errorf("build get request: %w", err)
	}
	headerKey, headerVal := s.scopeHeader()
	if headerKey == "" {
		return nil, fmt.Errorf("blob store not initialised: exactly one of orgID / ownerID required")
	}
	req.Header.Set(headerKey, headerVal)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("blob get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s", ErrBlobNotFound, handle)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("blob get: status %d: %s", resp.StatusCode, truncateForErr(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("blob get: read body: %w", err)
	}

	s.mu.Lock()
	if len(s.cache) >= cacheMaxEntries {
		s.cache = map[string][]byte{}
	}
	cp := make([]byte, len(body))
	copy(cp, body)
	s.cache[handle] = cp
	s.mu.Unlock()

	return body, nil
}

// Cleanup drops the in-process cache. The API-side rows are managed
// by the server-side TTL sweep (tool_output → 1 h), so there's
// nothing local to delete. Provided for API symmetry with the old
// disk-backed implementation.
func (s *BlobStore) Cleanup() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.cache = map[string][]byte{}
	s.mu.Unlock()
	return nil
}

// ErrBlobNotFound is returned by Get when the API responds 404 — both
// missing handles and cross-org reads collapse to this value because
// the API does not distinguish (existence isn't leakable). Sentinel so
// callers can pattern-match without scraping error text.
var ErrBlobNotFound = fmt.Errorf("blob not found")

// formatBlobToken builds the verbose `flo:blob:<handle>?size=N&type=M`
// representation. Used by the API server today and retained here for
// tests + any future fallback path that needs to reconstruct a token
// client-side.
func formatBlobToken(handle string, size int, mimeHint string) string {
	var sb strings.Builder
	sb.WriteString(BlobTokenPrefix)
	sb.WriteString(handle)
	sb.WriteString("?size=")
	sb.WriteString(strconv.Itoa(size))
	if mimeHint != "" {
		sb.WriteString("&type=")
		sb.WriteString(url.QueryEscape(mimeHint))
	}
	return sb.String()
}

// IsBlobToken reports whether s is an exact-shape blob token. The
// check is strict — it rejects strings that merely contain a token
// substring, so a sentence in a tool_result that mentions a token
// won't be re-interpreted as one.
func IsBlobToken(s string) bool {
	_, _, _, ok := ParseBlobToken(s)
	return ok
}

// ParseBlobToken extracts the handle plus metadata from a verbose
// token. Tolerates:
//   - Full form:        flo:blob:<handle>?size=N&type=M
//   - Without type:     flo:blob:<handle>?size=N
//   - Bare handle:      flo:blob:<handle>
//
// Returns ok=false if the prefix is missing, the handle isn't a
// 32-character lowercase hex string, or anything else departs from
// the expected shape.
func ParseBlobToken(s string) (handle string, size int, mime string, ok bool) {
	if !strings.HasPrefix(s, BlobTokenPrefix) {
		return "", 0, "", false
	}
	body := strings.TrimPrefix(s, BlobTokenPrefix)

	queryStart := strings.Index(body, "?")
	if queryStart < 0 {
		if !isHexHandle(body) {
			return "", 0, "", false
		}
		return body, 0, "", true
	}

	handle = body[:queryStart]
	if !isHexHandle(handle) {
		return "", 0, "", false
	}

	q, err := url.ParseQuery(body[queryStart+1:])
	if err != nil {
		return "", 0, "", false
	}
	if v := q.Get("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			size = n
		}
	}
	mime = q.Get("type")
	return handle, size, mime, true
}

// blobHandleHexLen is the canonical hex-encoded length of a handle:
// 16 random bytes = 32 hex characters. Pinned as a constant so the
// drift catches at compile/test time, not at parse time.
const blobHandleHexLen = 32

func isHexHandle(s string) bool {
	if len(s) != blobHandleHexLen {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// extractTokenFromUploadResponse pulls the canonical token out of
// the API's create-blob JSON. Kept as a tiny helper so a token
// shape change in the API is a one-line update here, not a scatter
// across Put callers.
func extractTokenFromUploadResponse(body []byte) (string, error) {
	// The full response shape is { handle, blob_token, size, mime,
	// purpose }; we only need blob_token.
	//
	// This MUST be a real JSON decode, not a substring scan. The API
	// renders the response with encoding/json's default HTML escaping,
	// so the token's "&" separator (…?size=N&type=mime) arrives on the
	// wire as the six literal characters `&`. A naive "read until
	// the next quote" scan captures that literal escape verbatim,
	// leaving a token whose query string can never be parsed by
	// URLSearchParams — the MIME hint is silently lost and the editor
	// falls back to guessing the media type from the output key name.
	// json.Unmarshal decodes the escape back to "&", keeping the token
	// canonical.
	var resp struct {
		// #nosec G101 -- field name for the API's upload response; the
		// value is an opaque flo:blob:... reference, not a credential.
		BlobToken string `json:"blob_token"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("upload response parse: %w: %s", err, truncateForErr(string(body)))
	}
	if resp.BlobToken == "" {
		return "", fmt.Errorf("upload response missing blob_token: %s", truncateForErr(string(body)))
	}
	if !IsBlobToken(resp.BlobToken) {
		return "", fmt.Errorf("upload response token malformed: %q", resp.BlobToken)
	}
	return resp.BlobToken, nil
}

// truncateForErr keeps error messages from spilling a multi-KB
// string into the logs.
func truncateForErr(s string) string {
	if len(s) > 64 {
		return s[:64] + "…"
	}
	return s
}

// _ avoids unused-import noise on hex when future changes need it.
// Kept here intentionally so the import block stays stable and any
// reviewer touching the file sees the hex helper as "available".
var _ = hex.EncodeToString

// blobTokenNameParam is the optional query parameter carrying a blob's
// original filename. Optional and additive: tokens minted before it existed
// simply have no name, and every parser here already tolerates unknown
// parameters, so nothing needs to be migrated.
//
// The name is a HINT for display and for naming an upload — never a path.
// It is sanitised on the way in and again on the way out, because a token can
// reach us from an LLM tool call and must not be able to steer a file write.
const blobTokenNameParam = "name"

// BlobTokenName returns the original filename a token carries, or "" when it
// carries none. The value is re-sanitised on read: a token is untrusted input.
func BlobTokenName(s string) string {
	if !strings.HasPrefix(s, BlobTokenPrefix) {
		return ""
	}
	body := strings.TrimPrefix(s, BlobTokenPrefix)
	i := strings.Index(body, "?")
	if i < 0 {
		return ""
	}
	q, err := url.ParseQuery(body[i+1:])
	if err != nil {
		return ""
	}
	return SanitiseFilename(q.Get(blobTokenNameParam))
}

// WithBlobTokenName returns token with a name= hint attached, replacing any
// name it already carries. Returns token unchanged when it is not a blob token
// or the name sanitises away to nothing.
func WithBlobTokenName(token, name string) string {
	clean := SanitiseFilename(name)
	if clean == "" || !IsBlobToken(token) {
		return token
	}

	body := strings.TrimPrefix(token, BlobTokenPrefix)
	handle, query := body, ""
	if i := strings.Index(body, "?"); i >= 0 {
		handle, query = body[:i], body[i+1:]
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		return token
	}
	q.Set(blobTokenNameParam, clean)

	// Encode() sorts keys, which would reorder size/type and churn every
	// token string. Rebuild in the canonical order instead so a token that
	// gains a name still matches the documented shape.
	var sb strings.Builder
	sb.WriteString(BlobTokenPrefix)
	sb.WriteString(handle)
	sb.WriteString("?size=")
	sb.WriteString(q.Get("size"))
	if v := q.Get("type"); v != "" {
		sb.WriteString("&type=")
		sb.WriteString(url.QueryEscape(v))
	}
	sb.WriteString("&" + blobTokenNameParam + "=")
	sb.WriteString(url.QueryEscape(clean))
	return sb.String()
}

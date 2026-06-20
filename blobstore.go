package core

// BlobStore — disk-backed key-value store for transparent off-loading
// of large tool-call outputs.
//
// Motivation. LLM tool loops cannot pass large binary payloads through
// the model context — a 400 KB base64 audio blob would either exhaust
// the context window or be hallucinated as a placeholder string by the
// model (see execution 0bab2c40-3905-463c-9103-dc164d381f69 for the
// canonical case: the model fabricated the literal string
// "generated_audio_base64" rather than carry the real bytes).
//
// The store sits between the action layer (which produces and consumes
// full values) and the AI orchestration layer (which only ever sees
// tokens). Actions remain entirely unaware of it. The AI sees compact
// `flo:blob:<handle>?size=N&type=mime` references in tool results and
// passes them verbatim to downstream tools; the executor resolves the
// reference back into the original bytes before invoking the action.
//
// Lifetime. One store per execution, scoped to the executor's working
// directory (which the runner creates per-execution and tears down on
// completion). Cleanup is therefore mostly redundant but provided so
// the contract is explicit.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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

// BlobTokenPrefix is the leading marker that identifies a string
// value as a blob reference. The prefix is verbose by design — it
// makes the token easy to spot in logs, model traces, and DB dumps;
// it also makes accidental collision with user content
// vanishingly unlikely.
const BlobTokenPrefix = "flo:blob:"

// BlobStore keeps a per-execution handle→file mapping plus the
// directory the blobs live in. It is safe for concurrent use.
type BlobStore struct {
	dir string

	mu      sync.Mutex
	handles map[string]string // handle -> on-disk path
}

// NewBlobStore creates a store rooted at <baseDir>/blobs. The
// directory is created lazily on first Put — an execution with no
// large outputs leaves no filesystem trace.
func NewBlobStore(baseDir string) *BlobStore {
	return &BlobStore{
		dir:     filepath.Join(baseDir, "blobs"),
		handles: map[string]string{},
	}
}

// Put writes value to disk and returns a verbose token of the form
//
//	flo:blob:<16-hex>?size=<bytes>&type=<mime>
//
// The mimeHint is optional but recommended — it lets the LLM reason
// about content type when choosing which tool to call next ("this is
// audio/mpeg, the next tool expects audio_base64 — same shape, pass
// it through"). An empty mimeHint produces a token without the type
// parameter; downstream resolution is unaffected.
func (s *BlobStore) Put(value []byte, mimeHint string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("blob store not initialised")
	}

	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return "", fmt.Errorf("create blob dir: %w", err)
	}

	handle, err := newHandle()
	if err != nil {
		return "", err
	}

	path := filepath.Join(s.dir, handle)
	// 0o640 — readable by the executor process and the runner that
	// spawned it, no broader. Blobs may contain credential-adjacent
	// data (TTS audio of a verification code, OCR'd images of an ID
	// document) so we treat the on-disk form as sensitive.
	if err := os.WriteFile(path, value, 0o640); err != nil {
		return "", fmt.Errorf("write blob %s: %w", handle, err)
	}

	s.mu.Lock()
	s.handles[handle] = path
	s.mu.Unlock()

	return formatBlobToken(handle, len(value), mimeHint), nil
}

// Get resolves a token back to the original bytes. It tolerates the
// full verbose token format (with query params) and bare-handle
// shortcuts in case the LLM strips the parameters when echoing.
//
// Returns an error wrapping ErrBlobNotFound when the handle is not
// known to this store — the caller is responsible for translating
// that into a user-facing error explaining how to wire tokens
// correctly.
func (s *BlobStore) Get(token string) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("blob store not initialised")
	}

	handle, _, _, ok := ParseBlobToken(token)
	if !ok {
		return nil, fmt.Errorf("not a blob token: %q", truncateForErr(token))
	}

	s.mu.Lock()
	path, exists := s.handles[handle]
	s.mu.Unlock()

	if !exists {
		// Fall back to a direct disk lookup. Handles are 16 hex
		// chars and live directly under s.dir, so this is safe
		// without traversal guarding — strict hex check prevents
		// any path component injection.
		if !isHexHandle(handle) {
			return nil, fmt.Errorf("%w: %s", ErrBlobNotFound, handle)
		}
		candidate := filepath.Join(s.dir, handle)
		if _, err := os.Stat(candidate); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrBlobNotFound, handle)
		}
		path = candidate
	}

	return os.ReadFile(path)
}

// Cleanup removes the on-disk blob directory and resets the in-
// memory map. The runner already tears down the per-execution
// working directory after the executor exits, so this is belt-and-
// braces — but harmless to call from a defer.
func (s *BlobStore) Cleanup() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.handles = map[string]string{}
	dir := s.dir
	s.mu.Unlock()
	return os.RemoveAll(dir)
}

// ErrBlobNotFound is returned by Get when a handle doesn't match
// any blob in this store. Sentinel so callers can pattern-match
// without scraping error text.
var ErrBlobNotFound = fmt.Errorf("blob not found")

// newHandle returns a fresh 16-hex-character handle. 8 bytes of
// randomness gives 64 bits of entropy — collision probability is
// negligible for the per-execution scope we operate in.
func newHandle() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate handle: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// formatBlobToken builds the verbose `flo:blob:<handle>?size=N&type=M`
// representation. Always includes size; only includes type when a
// hint is provided.
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

// IsBlobToken reports whether s is exact-shape blob token. The
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
// Returns ok=false if the prefix is missing, the handle isn't 16
// hex chars, or anything else departs from the expected shape.
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

func isHexHandle(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// truncateForErr keeps error messages from spilling a multi-KB
// string into the logs when ParseBlobToken is handed something
// unreasonable.
func truncateForErr(s string) string {
	if len(s) > 64 {
		return s[:64] + "…"
	}
	return s
}

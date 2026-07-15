package core

// BlobStore tests against an in-process stub HTTP server that mimics
// the API's M0 blob endpoints. We deliberately don't share code with
// the API repo — the stub is small enough that duplicating it keeps
// the tests independent of the API package's internals while still
// pinning the wire contract we need to round-trip with.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
)

// stubBlobServer is a minimal API-compatible store. It enforces
// "exactly one of X-Flomation-Org-Id / X-Flomation-Owner-Id" on every
// request and partitions storage by the kind+id so cross-scope tests
// confirm the 404-not-403 invariant without hitting production code.
type stubBlobServer struct {
	mu        sync.Mutex
	store     map[string][]byte // key: scope-key + ":" + handle (hex)
	storeMime map[string]string
	server    *httptest.Server
}

// resolveScope reads the (org-or-owner) headers and returns a
// scope-key string (e.g. "org:org-1" or "owner:user-andy"). Empty
// string means the caller violated the exactly-one rule.
func resolveScope(r *http.Request) string {
	orgID := r.Header.Get(OrgIDHeader)
	ownerID := r.Header.Get(OwnerIDHeader)
	if (orgID == "") == (ownerID == "") {
		return "" // both set or both empty — invalid
	}
	if orgID != "" {
		return "org:" + orgID
	}
	return "owner:" + ownerID
}

func newStubBlobServer(t *testing.T) *stubBlobServer {
	t.Helper()
	s := &stubBlobServer{
		store:     map[string][]byte{},
		storeMime: map[string]string{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/internal/blob", s.handleRoot)
	mux.HandleFunc("/api/v1/internal/blob/", s.handleByHandle)
	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

// keyFor scopes a handle within a (kind, id) tuple. Cross-scope
// isolation is modelled here so tests don't need to spin up two stub
// servers to verify it.
func (s *stubBlobServer) keyFor(scopeKey, handle string) string {
	return scopeKey + ":" + handle
}

func (s *stubBlobServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	scopeKey := resolveScope(r)
	if scopeKey == "" {
		http.Error(w, `{"error":"exactly one scope header required"}`, http.StatusBadRequest)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mime := r.FormValue("mime")
	if mime == "" {
		http.Error(w, `{"error":"mime required"}`, http.StatusBadRequest)
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	handle := make([]byte, 16)
	_, _ = rand.Read(handle)
	hh := hex.EncodeToString(handle)

	s.mu.Lock()
	s.store[s.keyFor(scopeKey, hh)] = body
	s.storeMime[s.keyFor(scopeKey, hh)] = mime
	s.mu.Unlock()

	token := formatBlobToken(hh, len(body), mime)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"handle":"%s","blob_token":"%s","size":%d,"mime":"%s","purpose":"%s"}`,
		hh, token, len(body), mime, r.FormValue("purpose"))
}

func (s *stubBlobServer) handleByHandle(w http.ResponseWriter, r *http.Request) {
	scopeKey := resolveScope(r)
	if scopeKey == "" {
		http.Error(w, `{"error":"exactly one scope header required"}`, http.StatusBadRequest)
		return
	}
	// Path is /api/v1/internal/blob/<handle>
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/internal/blob/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, `{"error":"missing handle"}`, http.StatusNotFound)
		return
	}
	handle := parts[0]

	switch r.Method {
	case http.MethodGet:
		s.mu.Lock()
		body, ok := s.store[s.keyFor(scopeKey, handle)]
		mime := s.storeMime[s.keyFor(scopeKey, handle)]
		s.mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"blob not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", mime)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	case http.MethodDelete:
		s.mu.Lock()
		_, ok := s.store[s.keyFor(scopeKey, handle)]
		if ok {
			delete(s.store, s.keyFor(scopeKey, handle))
			delete(s.storeMime, s.keyFor(scopeKey, handle))
		}
		s.mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"blob not found"}`, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// newTestBlobStore returns a BlobStore wired to a fresh stub server
// scoped to "org-1" / "exec-1". Tests that need cross-org behaviour
// build a second store with a different orgID against the same stub.
func newTestBlobStore(t *testing.T) (*BlobStore, *stubBlobServer) {
	srv := newStubBlobServer(t)
	store := NewBlobStore(http.DefaultClient, srv.server.URL, "org-1", "", "exec-1")
	return store, srv
}

func TestBlobStore_PutGetRoundTrip(t *testing.T) {
	RegisterTestingT(t)
	store, _ := newTestBlobStore(t)

	payload := []byte(strings.Repeat("X", 50000))
	token, err := store.Put(payload, "application/octet-stream")
	Expect(err).NotTo(HaveOccurred())
	Expect(token).To(HavePrefix(BlobTokenPrefix))
	Expect(token).To(ContainSubstring("size=50000"))
	Expect(token).To(ContainSubstring("type=application%2Foctet-stream"))

	got, err := store.Get(token)
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(payload))
}

func TestBlobStore_ParseBlobToken_Shapes(t *testing.T) {
	RegisterTestingT(t)

	// 32-character lowercase hex handle — the API's 16-byte format.
	const validHandle = "0123456789abcdef0123456789abcdef"
	const validHandle2 = "fedcba9876543210fedcba9876543210"
	const validHandle3 = "00112233445566778899aabbccddeeff"

	type tc struct {
		name   string
		input  string
		ok     bool
		handle string
		size   int
		mime   string
	}
	cases := []tc{
		{"full form", BlobTokenPrefix + validHandle + "?size=1024&type=audio%2Fmpeg",
			true, validHandle, 1024, "audio/mpeg"},
		{"without type", BlobTokenPrefix + validHandle2 + "?size=42",
			true, validHandle2, 42, ""},
		{"bare handle", BlobTokenPrefix + validHandle3,
			true, validHandle3, 0, ""},
		{"wrong prefix", "blob:" + validHandle,
			false, "", 0, ""},
		// The 16-character form is the *old* disk-backed format —
		// rejecting it loudly is part of the M1 contract upgrade.
		{"old 16-char handle rejected", BlobTokenPrefix + "0123456789abcdef",
			false, "", 0, ""},
		{"short handle", BlobTokenPrefix + "0123",
			false, "", 0, ""},
		{"non-hex handle", BlobTokenPrefix + "0123456789xxxxxxxxxxxxxxxxxxxxxx",
			false, "", 0, ""},
		{"empty", "", false, "", 0, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			RegisterTestingT(t)
			h, s, m, ok := ParseBlobToken(c.input)
			Expect(ok).To(Equal(c.ok))
			if c.ok {
				Expect(h).To(Equal(c.handle))
				Expect(s).To(Equal(c.size))
				Expect(m).To(Equal(c.mime))
			}
		})
	}
}

func TestBlobStore_GetUnknownHandle_ReturnsErrBlobNotFound(t *testing.T) {
	RegisterTestingT(t)
	store, _ := newTestBlobStore(t)

	// 32-character hex handle that was never uploaded.
	_, err := store.Get(BlobTokenPrefix + "00000000000000000000000000000000?size=0")
	Expect(err).To(HaveOccurred())
	Expect(err).To(MatchError(ContainSubstring("blob not found")))
}

func TestBlobStore_GetNonToken_RejectsBeforeNetworkCall(t *testing.T) {
	RegisterTestingT(t)
	store, _ := newTestBlobStore(t)

	_, err := store.Get("this is not a token")
	Expect(err).To(MatchError(ContainSubstring("not a blob token")))
}

func TestBlobStore_CrossOrgRead_Returns404(t *testing.T) {
	RegisterTestingT(t)
	srv := newStubBlobServer(t)

	storeA := NewBlobStore(http.DefaultClient, srv.server.URL, "org-A", "", "exec-1")
	storeB := NewBlobStore(http.DefaultClient, srv.server.URL, "org-B", "", "exec-1")

	tok, err := storeA.Put([]byte("from org A"), "text/plain")
	Expect(err).NotTo(HaveOccurred())

	// Same token, different org → not found. The stub mirrors the
	// production API's "collapse to ErrBlobNotFound" behaviour.
	_, err = storeB.Get(tok)
	Expect(err).To(HaveOccurred())
	Expect(err).To(MatchError(ContainSubstring("blob not found")))
}

func TestBlobStore_Cleanup_DropsCacheButKeepsServerRows(t *testing.T) {
	RegisterTestingT(t)
	store, _ := newTestBlobStore(t)

	tok, err := store.Put([]byte("hello"), "text/plain")
	Expect(err).NotTo(HaveOccurred())
	_, err = store.Get(tok) // primes the cache
	Expect(err).NotTo(HaveOccurred())

	Expect(store.Cleanup()).To(Succeed())

	// After cleanup, the server row still exists — we can Get again.
	got, err := store.Get(tok)
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal([]byte("hello")))
}

func TestBlobStore_GetCacheHit_AvoidsSecondRequest(t *testing.T) {
	RegisterTestingT(t)

	// Stand up a counting stub so we can verify the second Get is
	// served from the in-process cache without a network round trip.
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handle := make([]byte, 16)
			_, _ = rand.Read(handle)
			hh := hex.EncodeToString(handle)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"handle":"%s","blob_token":"%s","size":5,"mime":"text/plain","purpose":"tool_output"}`,
				hh, formatBlobToken(hh, 5, "text/plain"))
			return
		}
		calls++
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))
	t.Cleanup(srv.Close)

	store := NewBlobStore(http.DefaultClient, srv.URL, "org-1", "", "exec-1")
	tok, err := store.Put([]byte("hello"), "text/plain")
	Expect(err).NotTo(HaveOccurred())

	for i := 0; i < 3; i++ {
		got, err := store.Get(tok)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal([]byte("hello")))
	}
	Expect(calls).To(Equal(1)) // first Get hit the wire; next two cached
}

func TestBlobStore_PutWithoutAPIURL_Errors(t *testing.T) {
	RegisterTestingT(t)
	store := NewBlobStore(http.DefaultClient, "", "org-1", "", "exec-1")
	_, err := store.Put([]byte("x"), "text/plain")
	Expect(err).To(MatchError(ContainSubstring("apiURL")))
}

func TestBlobStore_PutWithoutOrgID_Errors(t *testing.T) {
	RegisterTestingT(t)
	srv := newStubBlobServer(t)
	store := NewBlobStore(http.DefaultClient, srv.server.URL, "", "", "exec-1")
	_, err := store.Put([]byte("x"), "text/plain")
	Expect(err).To(MatchError(ContainSubstring("orgID")))
}

func TestBlobStore_PutPropagatesServerError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"quota exceeded"}`, http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	store := NewBlobStore(http.DefaultClient, srv.URL, "org-1", "", "exec-1")
	_, err := store.Put([]byte("x"), "text/plain")
	Expect(err).To(MatchError(ContainSubstring("status 429")))
}

func TestBlobStore_IsBlobToken_RejectsSubstringMatches(t *testing.T) {
	RegisterTestingT(t)
	Expect(IsBlobToken("flo:blob:0123456789abcdef0123456789abcdef")).To(BeTrue())
	Expect(IsBlobToken("here is flo:blob:0123456789abcdef0123456789abcdef inside")).To(BeFalse())
	Expect(IsBlobToken("")).To(BeFalse())
	// 16-char handle from the pre-M1 format must read as not-a-token.
	Expect(IsBlobToken("flo:blob:0123456789abcdef")).To(BeFalse())
}

func TestBlobStore_FormatBlobToken_EmitsExpectedShape(t *testing.T) {
	RegisterTestingT(t)
	tok := formatBlobToken("0123456789abcdef0123456789abcdef", 100, "image/png")
	Expect(tok).To(Equal("flo:blob:0123456789abcdef0123456789abcdef?size=100&type=image%2Fpng"))

	// Empty mimeHint → no &type= suffix. Important for downstream
	// parsers (the AI may strip ampersands when echoing).
	tokNoMime := formatBlobToken("0123456789abcdef0123456789abcdef", 100, "")
	Expect(tokNoMime).To(Equal("flo:blob:0123456789abcdef0123456789abcdef?size=100"))
	Expect(tokNoMime).NotTo(ContainSubstring("type="))
}

// TestExtractTokenFromUploadResponse_DecodesHTMLEscapedAmpersand pins the
// regression where the API's blob-upload response — rendered with
// encoding/json's default HTML escaping — encodes the token's "&" separator
// as the literal escape "&". A substring scan captured that escape
// verbatim, producing a token whose "?size=N&type=mime" query string could
// not be parsed downstream (the editor lost the MIME hint and mislabelled
// the media). The extractor must JSON-decode the value so "&" is restored.
func TestExtractTokenFromUploadResponse_DecodesHTMLEscapedAmpersand(t *testing.T) {
	RegisterTestingT(t)
	handle := strings.Repeat("a", 32)
	canonical := "flo:blob:" + handle + "?size=88794&type=image%2Fpng"

	// Reproduce the exact wire bytes the API emits: encoding/json with HTML
	// escaping ON (gin's c.JSON default) renders the "&" separator as its
	// & escape. We build it with the same encoder rather than typing the
	// escape by hand, so the fixture can never drift from real behaviour.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	Expect(enc.Encode(map[string]string{"blob_token": canonical})).To(Succeed())
	body := buf.Bytes()
	// Sanity: the wire form really did escape the separator — no literal "&".
	Expect(bytes.ContainsRune(body, '&')).To(BeFalse())

	token, err := extractTokenFromUploadResponse(body)
	Expect(err).To(BeNil())
	// The extractor must have JSON-decoded the escape back to a real "&", so
	// the query string parses and the MIME hint survives downstream.
	Expect(token).To(Equal(canonical))
	Expect(strings.Contains(token, "&type=image%2Fpng")).To(BeTrue())
}

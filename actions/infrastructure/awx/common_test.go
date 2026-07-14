package awx

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

func strInput(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}

func secretInput(name string, val interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeSecret, Value: val}
}

// tokenAuth is the credential every HTTP test uses unless it is testing auth.
//
// The token is a realistic 30-character AWX PAT rather than a toy like "tok" on
// purpose: Redact scrubs a credential with a plain substring replace, so a
// degenerate secret would corrupt the very error messages these tests assert on
// (a token of "tok" turns "The API token may be wrong" into "The API REDACTEDen
// may be wrong"). That over-redaction is the deliberate, safe direction — see
// Redact — so the fixture models the real thing.
func tokenAuth(base string) Auth {
	return Auth{BaseURL: strings.TrimRight(base, "/"), Method: AuthMethodToken, Token: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"}
}

// awxServer stands up a server that answers the API-root discovery probe exactly
// as a real upstream AWX 24.6.1 does, and delegates everything else to h.
func awxServer(h http.HandlerFunc) *httptest.Server {
	ResetAPIRootCacheForTest()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			_, _ = w.Write([]byte(`{"description":"AWX REST API","current_version":"/api/v2/","available_versions":{"v2":"/api/v2/"},"oauth2":"/api/o/"}`))
		case "/api/v2/me/":
			w.Header().Set("X-API-Product-Name", "AWX")
			w.Header().Set("X-API-Product-Version", "24.6.1")
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1,"username":"admin","is_superuser":true}]}`))
		default:
			h(w, r)
		}
	}))
}

// ---------------------------------------------------------------------------
// URL handling
// ---------------------------------------------------------------------------

func TestNormaliseBaseURL(t *testing.T) {
	RegisterTestingT(t)

	cases := map[string]string{
		"https://awx.example.com":           "https://awx.example.com",
		"https://awx.example.com/":          "https://awx.example.com",
		"http://192.168.80.27":              "http://192.168.80.27",
		"http://192.168.80.27:8043/":        "http://192.168.80.27:8043",
		"awx.example.com":                   "https://awx.example.com", // bare host → https
		"https://host/awx/":                 "https://host/awx",        // context path preserved
		"https://awx.example.com/?x=1#frag": "https://awx.example.com", // query/fragment stripped
		"https://user:pass@awx.example.com": "https://awx.example.com", // userinfo dropped
		"  https://awx.example.com  ":       "https://awx.example.com", // trimmed

		// Operators paste the API URL out of the browser bar constantly.
		"https://awx.example.com/api":                "https://awx.example.com",
		"https://awx.example.com/api/":               "https://awx.example.com",
		"https://awx.example.com/api/v2/":            "https://awx.example.com",
		"https://awx.example.com/api/controller/v2/": "https://awx.example.com",
		"https://host/awx/api/v2/":                   "https://host/awx", // suffix stripped, context path kept
	}
	for in, want := range cases {
		got, err := NormaliseBaseURL(in)
		Expect(err).To(BeNil(), in)
		Expect(got).To(Equal(want), in)
	}

	for _, bad := range []string{"", "   ", "ftp://awx", "http://"} {
		_, err := NormaliseBaseURL(bad)
		Expect(err).To(HaveOccurred(), bad)
	}
}

// TestEnsureTrailingSlash pins the guard against Django's APPEND_SLASH: a
// slash-less POST is 301'd, and Go's client re-issues a redirected POST as a GET
// with NO BODY — a launch that silently does nothing.
func TestEnsureTrailingSlash(t *testing.T) {
	RegisterTestingT(t)

	cases := map[string]string{
		"jobs":                    "jobs/",
		"jobs/":                   "jobs/",
		"job_templates/7/launch":  "job_templates/7/launch/",
		"job_templates/7/launch/": "job_templates/7/launch/",

		// The query string must be split off first, or the slash lands on the query.
		"jobs?id=4":                 "jobs/?id=4",
		"jobs/?id=4":                "jobs/?id=4",
		"jobs/5/stdout?format=txt":  "jobs/5/stdout/?format=txt",
		"jobs/5/stdout/?format=txt": "jobs/5/stdout/?format=txt",
	}
	for in, want := range cases {
		Expect(ensureTrailingSlash(in)).To(Equal(want), in)
	}
}

// ---------------------------------------------------------------------------
// ★ API-root discovery
// ---------------------------------------------------------------------------

func TestResolveAPIRootUpstreamAWX(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s", r.URL.Path)
	})
	defer srv.Close()

	root, err := ResolveAPIRoot(context.Background(), tokenAuth(srv.URL))
	Expect(err).To(BeNil())
	Expect(root.Prefix).To(Equal("/api/v2/"))
	Expect(root.Product).To(Equal("AWX"))
	Expect(root.Version).To(Equal("24.6.1"))
}

// TestResolveAPIRootAAPGateway covers the AAP 2.5 platform gateway, whose /api/
// root answers 200 but carries NO available_versions (proven by awx#16054, where
// `awx login` gets its 200 and then crashes on the missing attribute). We key on
// that ABSENCE — never on a gateway field name we would be guessing at.
func TestResolveAPIRootAAPGateway(t *testing.T) {
	RegisterTestingT(t)
	ResetAPIRootCacheForTest()

	var hitLegacy int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			_, _ = w.Write([]byte(`{"description":"AAP gateway","apis":{"gateway":"/api/gateway/v1/"}}`))
		case "/api/v2/me/":
			atomic.AddInt32(&hitLegacy, 1)
			w.WriteHeader(http.StatusNotFound)
		case "/api/controller/v2/me/":
			w.Header().Set("X-API-Product-Name", "Red Hat Ansible Automation Platform")
			w.Header().Set("X-API-Product-Version", "2.5")
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1,"username":"admin"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	root, err := ResolveAPIRoot(context.Background(), tokenAuth(srv.URL))
	Expect(err).To(BeNil())
	Expect(root.Prefix).To(Equal("/api/controller/v2/"))
	Expect(root.Product).To(Equal("Red Hat Ansible Automation Platform"))

	// The banner already told us it was a gateway, so the legacy root is not even
	// tried — but the sweep below proves we would still find it if it had not.
	Expect(atomic.LoadInt32(&hitLegacy)).To(Equal(int32(0)))
}

// TestResolveAPIRootSweepsWhenTheBannerIsUnhelpful is the safety net: when GET
// /api/ tells us nothing (a proxy 500s it, say), we fall back to sweeping both
// candidate roots with an authenticated me/, which is authoritative.
func TestResolveAPIRootSweepsWhenTheBannerIsUnhelpful(t *testing.T) {
	RegisterTestingT(t)
	ResetAPIRootCacheForTest()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			w.WriteHeader(http.StatusInternalServerError)
		case "/api/v2/me/":
			w.WriteHeader(http.StatusNotFound) // wrong prefix — keep sweeping
		case "/api/controller/v2/me/":
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	root, err := ResolveAPIRoot(context.Background(), tokenAuth(srv.URL))
	Expect(err).To(BeNil())
	Expect(root.Prefix).To(Equal("/api/controller/v2/"))
}

// ★ TestResolveAPIRootBadCredentialDoesNotSweep is the most important test in
// this file. A 401 from me/ means the PREFIX IS RIGHT and the CREDENTIAL IS
// WRONG. Sweeping on it would turn "your token is invalid" into "we cannot find
// AWX" — the single most misleading failure this node could produce.
func TestResolveAPIRootBadCredentialDoesNotSweep(t *testing.T) {
	RegisterTestingT(t)
	ResetAPIRootCacheForTest()

	var hitSecondCandidate int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			_, _ = w.Write([]byte(`{"available_versions":{"v2":"/api/v2/"}}`))
		case "/api/v2/me/":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"detail":"Authentication credentials were not provided."}`))
		case "/api/controller/v2/me/":
			atomic.AddInt32(&hitSecondCandidate, 1)
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	_, err := ResolveAPIRoot(context.Background(), tokenAuth(srv.URL))
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("rejected the credential"))
	Expect(err.Error()).To(ContainSubstring("token"))
	Expect(err.Error()).NotTo(ContainSubstring("Could not find"))

	// ★ The second candidate must NEVER be requested.
	Expect(atomic.LoadInt32(&hitSecondCandidate)).To(Equal(int32(0)),
		"a 401 means the credential is wrong, not that the prefix is — sweeping hides that")
}

func TestResolveAPIRootOverrideSkipsProbe(t *testing.T) {
	RegisterTestingT(t)
	ResetAPIRootCacheForTest()

	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a := tokenAuth(srv.URL)
	a.APIPrefix = "api/controller/v2" // no slashes either side — normalised
	root, err := ResolveAPIRoot(context.Background(), a)
	Expect(err).To(BeNil())
	Expect(root.Prefix).To(Equal("/api/controller/v2/"))
	Expect(atomic.LoadInt32(&requests)).To(Equal(int32(0)), "an operator override must not probe at all")
}

func TestResolveAPIRootCaches(t *testing.T) {
	RegisterTestingT(t)

	var banners, mes int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			atomic.AddInt32(&banners, 1)
			_, _ = w.Write([]byte(`{"available_versions":{"v2":"/api/v2/"}}`))
		case "/api/v2/me/":
			atomic.AddInt32(&mes, 1)
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	ResetAPIRootCacheForTest()

	a := tokenAuth(srv.URL)
	for i := 0; i < 5; i++ {
		root, err := ResolveAPIRoot(context.Background(), a)
		Expect(err).To(BeNil())
		Expect(root.Prefix).To(Equal("/api/v2/"))
	}
	Expect(atomic.LoadInt32(&banners)).To(Equal(int32(1)), "discovery must happen once per base URL")
	Expect(atomic.LoadInt32(&mes)).To(Equal(int32(1)))
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

func TestBearerAndBasicHeaders(t *testing.T) {
	RegisterTestingT(t)

	var gotAuth, gotAccept, gotContentType, gotBody string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotContentType = r.Header.Get("Content-Type")
		buf := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			_, _ = r.Body.Read(buf)
		}
		gotBody = string(buf)
		_, _ = w.Write([]byte(`{"id":1}`))
	})
	defer srv.Close()

	// Bearer (RFC 6750) — the default.
	auth, err := GetAuth([]*core.Connection{
		strInput("awx_url", srv.URL),
		secretInput("api_token", "s3cr3t"),
	})
	Expect(err).To(BeNil())
	Expect(auth.Method).To(Equal(AuthMethodToken), "a blank auth_method must mean token")

	resp, err := Do(context.Background(), auth, http.MethodPost, "job_templates/7/launch/", map[string]interface{}{"extra_vars": map[string]interface{}{"a": 1}})
	Expect(err).To(BeNil())
	Expect(resp.StatusCode).To(Equal(200))
	Expect(gotAuth).To(Equal("Bearer s3cr3t"))
	Expect(gotAccept).To(Equal("application/json"))
	// AWX registers only JSONParser — a form-encoded body is a flat 415.
	Expect(gotContentType).To(Equal("application/json"))
	Expect(gotBody).To(ContainSubstring(`"extra_vars"`))

	// Basic — the fallback.
	ResetAPIRootCacheForTest()
	basic, err := GetAuth([]*core.Connection{
		strInput("awx_url", srv.URL),
		strInput("auth_method", AuthMethodBasic),
		strInput("awx_username", "admin"),
		secretInput("awx_password", "pw"),
	})
	Expect(err).To(BeNil())

	_, err = Do(context.Background(), basic, http.MethodGet, "ping/", nil)
	Expect(err).To(BeNil())
	Expect(gotAuth).To(Equal("Basic " + base64.StdEncoding.EncodeToString([]byte("admin:pw"))))
}

// TestDoForcesTheTrailingSlash proves the request that reaches AWX carries the
// slash Django demands, so a POST is never 301'd into a body-less GET.
func TestDoForcesTheTrailingSlash(t *testing.T) {
	RegisterTestingT(t)

	var gotPath, gotQuery, gotMethod string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotMethod = r.URL.Path, r.URL.RawQuery, r.Method
		_, _ = w.Write([]byte(`{}`))
	})
	defer srv.Close()

	_, err := Do(context.Background(), tokenAuth(srv.URL), http.MethodPost, "job_templates/7/launch", map[string]interface{}{})
	Expect(err).To(BeNil())
	Expect(gotMethod).To(Equal(http.MethodPost))
	Expect(gotPath).To(Equal("/api/v2/job_templates/7/launch/"))

	_, err = Do(context.Background(), tokenAuth(srv.URL), http.MethodGet, "jobs?id=4", nil)
	Expect(err).To(BeNil())
	Expect(gotPath).To(Equal("/api/v2/jobs/"))
	Expect(gotQuery).To(Equal("id=4"))
}

// ---------------------------------------------------------------------------
// Error translation
// ---------------------------------------------------------------------------

func TestCheckResponseTranslations(t *testing.T) {
	RegisterTestingT(t)

	token := Auth{Method: AuthMethodToken, Token: "tok"}
	basic := Auth{Method: AuthMethodBasic, Username: "admin", Password: "pw"}

	// Success, and the acceptable-codes escape hatch.
	Expect(CheckResponse(token, &Response{StatusCode: 200})).To(BeNil())
	Expect(CheckResponse(token, &Response{StatusCode: 204})).To(BeNil())
	Expect(CheckResponse(token, &Response{StatusCode: 202}, 202)).To(BeNil())
	Expect(CheckResponse(token, &Response{StatusCode: 405}, 202)).To(HaveOccurred())

	// 401 — the token is wrong, and AWX shows a token only once.
	err := CheckResponse(token, &Response{StatusCode: 401, Body: []byte(`{"detail":"Authentication credentials were not provided."}`)})
	Expect(err.Error()).To(ContainSubstring("rejected the credential (HTTP 401)"))
	Expect(err.Error()).To(ContainSubstring("shown only once"))

	// 401 on Basic — name the AUTH_BASIC_ENABLED trap, which is why Basic is a
	// fallback and never the only mode.
	err = CheckResponse(basic, &Response{StatusCode: 401})
	Expect(err.Error()).To(ContainSubstring("AUTH_BASIC_ENABLED"))

	// ★ 403 on a POST to …/launch/ — a READ-scoped token authenticates fine on
	// every GET and fails only here.
	err = CheckResponse(token, &Response{
		StatusCode: 403,
		Method:     http.MethodPost,
		URL:        "https://awx/api/v2/job_templates/7/launch/",
		Body:       []byte(`{"detail":"You do not have permission to perform this action."}`),
	})
	Expect(err.Error()).To(ContainSubstring("refused the launch (HTTP 403)"))
	Expect(err.Error()).To(ContainSubstring("READ-scoped"))
	Expect(err.Error()).To(ContainSubstring("Execute role"))

	// A 403 anywhere else is a plain permission problem, and keeps AWX's words.
	err = CheckResponse(token, &Response{
		StatusCode: 403,
		Method:     http.MethodDelete,
		URL:        "https://awx/api/v2/credentials/1/",
		Body:       []byte(`{"detail":"Deletion not allowed for managed credentials"}`),
	})
	Expect(err.Error()).To(ContainSubstring("HTTP 403"))
	Expect(err.Error()).To(ContainSubstring("managed credentials"))
	Expect(err.Error()).NotTo(ContainSubstring("READ-scoped"))

	// 404 — AWX hides objects you cannot SEE behind a 404, so this must never be
	// reported as "deleted".
	err = CheckResponse(token, &Response{StatusCode: 404, Body: []byte(`{"detail":"Not found."}`)})
	Expect(err.Error()).To(ContainSubstring("404"))
	Expect(err.Error()).To(ContainSubstring("permission to see it"))

	// 409 with active_jobs — retryable, not a generic failure.
	err = CheckResponse(token, &Response{
		StatusCode: 409,
		Body:       []byte(`{"error":"Resource is being used by running jobs.","active_jobs":[{"type":"job","id":12}]}`),
	})
	Expect(err.Error()).To(ContainSubstring("still running"))
	Expect(err.Error()).To(ContainSubstring("job 12"))

	// A DRF field dict is flattened, with a stable key order.
	err = CheckResponse(token, &Response{
		StatusCode: 400,
		Body:       []byte(`{"name":["This field is required."],"inventory":["No valid inventory."]}`),
	})
	Expect(err.Error()).To(ContainSubstring("AWX API error (400)"))
	Expect(err.Error()).To(ContainSubstring("inventory: No valid inventory."))
	Expect(err.Error()).To(ContainSubstring("name: This field is required."))
	Expect(strings.Index(err.Error(), "inventory:")).To(BeNumerically("<", strings.Index(err.Error(), "name:")))

	// The 400 a bare survey launch produces.
	err = CheckResponse(token, &Response{
		StatusCode: 400,
		Body:       []byte(`{"variables_needed_to_start":["'stopandrebuilt' value missing","'target_hosts' value missing"]}`),
	})
	Expect(err.Error()).To(ContainSubstring("'stopandrebuilt' value missing"))

	// A non-field error.
	err = CheckResponse(token, &Response{StatusCode: 400, Body: []byte(`{"__all__":["Cannot relaunch slice workflow job."]}`)})
	Expect(err.Error()).To(ContainSubstring("Cannot relaunch slice workflow job."))

	// An HTML page from a proxy in front of AWX is reduced to a hint, not echoed.
	err = CheckResponse(token, &Response{StatusCode: 502, Body: []byte("<html><body><h1>502 Bad Gateway</h1> nginx</body></html>")})
	Expect(err.Error()).To(ContainSubstring("502"))
	Expect(err.Error()).To(ContainSubstring("Bad Gateway"))
	Expect(err.Error()).NotTo(ContainSubstring("<h1>"))
}

func TestRedactStripsTokenPasswordAndBasicBlob(t *testing.T) {
	RegisterTestingT(t)

	a := Auth{Method: AuthMethodBasic, Username: "admin", Password: "hunter2", Token: "tok-abc-123"}
	blob := base64.StdEncoding.EncodeToString([]byte("admin:hunter2"))

	msg := fmt.Sprintf("Get https://awx/api/: token tok-abc-123 password hunter2 basic %s failed", blob)
	got := Redact(a, msg)

	Expect(got).NotTo(ContainSubstring("tok-abc-123"))
	Expect(got).NotTo(ContainSubstring("hunter2"))
	Expect(got).NotTo(ContainSubstring(blob))
	Expect(strings.Count(got, "REDACTED")).To(Equal(3))
	Expect(got).To(ContainSubstring("https://awx/api/"))

	// An empty credential must not turn every empty string into REDACTED.
	Expect(Redact(Auth{}, "plain message")).To(Equal("plain message"))
}

// TestCheckResponseRedactsTheCredential proves the scrub is wired into the error
// path — an error in a flow's output is an error in the audit log.
func TestCheckResponseRedactsTheCredential(t *testing.T) {
	RegisterTestingT(t)

	a := Auth{Method: AuthMethodToken, Token: "leak-me"}
	err := CheckResponse(a, &Response{StatusCode: 400, Body: []byte(`{"detail":"bad token leak-me supplied"}`)})
	Expect(err.Error()).NotTo(ContainSubstring("leak-me"))
	Expect(err.Error()).To(ContainSubstring("REDACTED"))
}

// ---------------------------------------------------------------------------
// Job records
// ---------------------------------------------------------------------------

// TestDecodeArtifactsHandlesNoLogString covers the type instability that breaks a
// naive client: job.artifacts is EITHER a JSON object OR the literal string
// "$hidden due to Ansible no_log flag$", so unmarshalling straight into a
// map[string]interface{} FAILS on any job whose set_stats was no_log.
func TestDecodeArtifactsHandlesNoLogString(t *testing.T) {
	RegisterTestingT(t)

	obj := DecodeArtifacts(map[string]interface{}{"build_id": "42"})
	Expect(obj).To(Equal(map[string]interface{}{"build_id": "42"}))

	hidden := DecodeArtifacts("$hidden due to Ansible no_log flag$")
	Expect(hidden).To(Equal("$hidden due to Ansible no_log flag$"))
	_, isMap := hidden.(map[string]interface{})
	Expect(isMap).To(BeFalse())

	Expect(DecodeArtifacts(nil)).To(Equal(map[string]interface{}{}))
}

// ★ TestLaunch201PolymorphismSlicedTemplateIsWorkflowJob: a job template with
// job_slice_count > 1 answers the launch with a WORKFLOW JOB — no "job" key at
// all — and polling /jobs/{id}/ for it 404s on an id that plainly exists.
func TestLaunch201PolymorphismSlicedTemplateIsWorkflowJob(t *testing.T) {
	RegisterTestingT(t)

	// An ordinary job-template launch.
	id, kind, err := LaunchedJob(map[string]interface{}{
		"job": float64(42), "id": float64(42), "type": "job", "status": "pending",
	}, JobKindJob)
	Expect(err).To(BeNil())
	Expect(id).To(Equal(int64(42)))
	Expect(kind).To(Equal(JobKindJob))

	// ★ A SLICED job template: type is workflow_job and there is no "job" key.
	id, kind, err = LaunchedJob(map[string]interface{}{
		"workflow_job": float64(99), "id": float64(99), "type": "workflow_job", "is_sliced_job": true,
	}, JobKindJob)
	Expect(err).To(BeNil())
	Expect(id).To(Equal(int64(99)))
	Expect(kind).To(Equal(JobKindWorkflowJob), "a sliced launch must be polled at /workflow_jobs/, not /jobs/")
	Expect(JobKindPaths[kind]).To(Equal("workflow_jobs/"))

	// A workflow RELAUNCH adds no extra key at all — the id is only in "id".
	id, kind, err = LaunchedJob(map[string]interface{}{"id": float64(77), "type": "workflow_job"}, JobKindWorkflowJob)
	Expect(err).To(BeNil())
	Expect(id).To(Equal(int64(77)))
	Expect(kind).To(Equal(JobKindWorkflowJob))

	// An ad-hoc relaunch, a project sync and an inventory sync each use their own key.
	id, kind, _ = LaunchedJob(map[string]interface{}{"ad_hoc_command": float64(5), "type": "ad_hoc_command"}, JobKindJob)
	Expect(id).To(Equal(int64(5)))
	Expect(kind).To(Equal(JobKindAdHocCommand))

	id, kind, _ = LaunchedJob(map[string]interface{}{"project_update": float64(8), "id": float64(8), "type": "project_update"}, JobKindProjectUpdate)
	Expect(id).To(Equal(int64(8)))
	Expect(kind).To(Equal(JobKindProjectUpdate))

	// No id at all is an error, not a silent zero.
	_, _, err = LaunchedJob(map[string]interface{}{"type": "job"}, JobKindJob)
	Expect(err).To(HaveOccurred())
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

// TestListFollowsRelativeNext pins the fact that AWX overrides DRF's
// get_next_link to use request.get_full_path(), so next/previous are RELATIVE
// PATHS, not absolute URLs. Treating one as a URL and handing it to the client
// verbatim fails outright.
func TestListFollowsRelativeNext(t *testing.T) {
	RegisterTestingT(t)

	var paths []string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.String())
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`{"count":3,"next":null,"previous":"/api/v2/jobs/?page_size=2","results":[{"id":3}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"count":3,"next":"/api/v2/jobs/?page=2&page_size=2","previous":null,"results":[{"id":1},{"id":2}]}`))
	})
	defer srv.Close()

	a := tokenAuth(srv.URL)

	// A single page: hasMore reports that rows remain.
	items, count, hasMore, err := List(context.Background(), a, "jobs/", nil, false)
	Expect(err).To(BeNil())
	Expect(items).To(HaveLen(2))
	Expect(count).To(Equal(3), "count is AWX's total, not the number returned")
	Expect(hasMore).To(BeTrue())
	Expect(paths).To(HaveLen(1))

	// Return All follows the RELATIVE next, resolved against the base.
	paths = nil
	items, count, hasMore, err = List(context.Background(), a, "jobs/", nil, true)
	Expect(err).To(BeNil())
	Expect(items).To(HaveLen(3))
	Expect(count).To(Equal(3))
	Expect(hasMore).To(BeFalse())
	Expect(paths).To(HaveLen(2))
	Expect(paths[1]).To(ContainSubstring("/api/v2/jobs/"))
	Expect(paths[1]).To(ContainSubstring("page=2"))
}

// TestListClampsPageSize: AWX's MAX_PAGE_SIZE clamp is SILENT, so a caller that
// asked for 1000 and trusted the answer would quietly miss rows.
func TestListClampsPageSize(t *testing.T) {
	RegisterTestingT(t)

	var query string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
	})
	defer srv.Close()

	q := map[string]string{"page_size": "1000"}
	values := make(map[string][]string, len(q))
	for k, v := range q {
		values[k] = []string{v}
	}
	_, _, _, err := List(context.Background(), tokenAuth(srv.URL), "jobs/", values, false)
	Expect(err).To(BeNil())
	Expect(query).To(ContainSubstring("page_size=200"))

	Expect(ClampPageSize(0, false)).To(Equal(DefaultPageSize))
	Expect(ClampPageSize(0, true)).To(Equal(DefaultPageSize))
	Expect(ClampPageSize(25, true)).To(Equal(25))
	Expect(ClampPageSize(9999, true)).To(Equal(MaxPageSize))
}

// TestResolveNextRefusesAForeignHost: an absolute next link pointing elsewhere
// would walk our bearer token to a server of the AWX's choosing.
func TestResolveNextRefusesAForeignHost(t *testing.T) {
	RegisterTestingT(t)

	got, err := resolveNext("https://awx.example.com", "/api/v2/jobs/?page=2")
	Expect(err).To(BeNil())
	Expect(got).To(Equal("https://awx.example.com/api/v2/jobs/?page=2"))

	// A context path lives in the next link itself, so joining on the ORIGIN is right.
	got, err = resolveNext("https://host/awx", "/awx/api/v2/jobs/?page=2")
	Expect(err).To(BeNil())
	Expect(got).To(Equal("https://host/awx/api/v2/jobs/?page=2"))

	got, err = resolveNext("https://awx.example.com", "https://awx.example.com/api/v2/jobs/?page=2")
	Expect(err).To(BeNil())
	Expect(got).To(Equal("https://awx.example.com/api/v2/jobs/?page=2"))

	_, err = resolveNext("https://awx.example.com", "https://evil.example.net/api/v2/jobs/?page=2")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("different host"))
}

// ---------------------------------------------------------------------------
// Stdout
// ---------------------------------------------------------------------------

// ★ TestStdoutTooLargeSentenceIsAnError: over STDOUT_MAX_BYTES_DISPLAY, AWX
// answers HTTP 200 whose BODY IS AN ENGLISH SENTENCE. A naive client stores that
// sentence as the playbook output.
func TestStdoutTooLargeSentenceIsAnError(t *testing.T) {
	RegisterTestingT(t)

	var gotQuery string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("Standard Output too large to display (2097152 bytes), only download supported for sizes over 1048576 bytes."))
	})
	defer srv.Close()

	text, truncated, err := FetchStdout(context.Background(), tokenAuth(srv.URL), JobKindJob, 5, 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Standard Output too large"))
	Expect(text).To(BeEmpty(), "the apology must never be stored as the playbook's output")
	Expect(truncated).To(BeFalse())

	// The format parameter is mandatory: without it a bare GET returns the
	// browsable API's HTML page, and ?format=txt is the capped variant.
	Expect(gotQuery).To(Equal("format=txt_download"))
}

func TestFetchStdoutTruncatesAtMaxBytes(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 100)))
	})
	defer srv.Close()

	text, truncated, err := FetchStdout(context.Background(), tokenAuth(srv.URL), JobKindJob, 5, 10)
	Expect(err).To(BeNil())
	Expect(text).To(Equal(strings.Repeat("x", 10)))
	Expect(truncated).To(BeTrue())

	text, truncated, err = FetchStdout(context.Background(), tokenAuth(srv.URL), JobKindJob, 5, 0)
	Expect(err).To(BeNil())
	Expect(text).To(HaveLen(100))
	Expect(truncated).To(BeFalse())

	// A workflow job has no output of its own — say so rather than 404ing.
	_, _, err = FetchStdout(context.Background(), tokenAuth(srv.URL), JobKindWorkflowJob, 5, 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("workflow job has no output"))
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// ★ TestOptionalStringOnUnsetSecretIsEmpty is the "<nil>" regression: on a Secret
// or Code input, core.Connection.String() renders a nil value as the LITERAL
// STRING "<nil>", so an unset API token reads back as non-empty and sails through
// the required-field check.
func TestOptionalStringOnUnsetSecretIsEmpty(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		secretInput("api_token", nil),
		{Name: "awx_password", Type: core.ConnectionTypeSecret},
		strInput("awx_url", "  https://awx.example.com  "),
	}

	Expect(OptionalString("api_token", inputs)).To(Equal(""), `an unset secret must not read back as "<nil>"`)
	Expect(OptionalString("awx_password", inputs)).To(Equal(""))
	Expect(OptionalString("missing", inputs)).To(Equal(""))
	Expect(OptionalString("awx_url", inputs)).To(Equal("https://awx.example.com"))

	// The consequence: GetAuth must refuse, rather than send "Bearer <nil>".
	_, err := GetAuth(inputs)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("API Token is required"))
}

func TestIntAndBoolInputs(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		// A live dropdown writes an AWX id into a STRING input.
		strInput("job_template_id", "7"),
		{Name: "page_size", Type: core.ConnectionTypeInteger, Value: float64(25)},
		{Name: "forks", Type: core.ConnectionTypeInteger, Value: nil},
		strInput("bad_id", "not-a-number"),
		// A variable-bound checkbox arrives as the STRING the substitution pass wrote.
		{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Value: "true"},
		{Name: "wait_for_completion", Type: core.ConnectionTypeBoolean, Value: true},
		{Name: "diff_mode", Type: core.ConnectionTypeBoolean, Value: ""},
	}

	id, err := RequiredInt("job_template_id", "Job Template", inputs)
	Expect(err).To(BeNil())
	Expect(id).To(Equal(int64(7)))

	n, ok := OptionalInt("page_size", inputs)
	Expect(ok).To(BeTrue())
	Expect(n).To(Equal(25))

	_, ok = OptionalInt("forks", inputs)
	Expect(ok).To(BeFalse(), "an unset integer must read as unset, not as 0")

	_, err = RequiredInt("missing", "Inventory", inputs)
	Expect(err.Error()).To(ContainSubstring("Inventory is required"))

	_, err = RequiredInt("bad_id", "Host", inputs)
	Expect(err.Error()).To(ContainSubstring("whole number"))

	Expect(BoolInput("confirm_destructive", inputs)).To(BeTrue(), `a variable-bound checkbox arrives as the string "true"`)
	Expect(BoolInput("wait_for_completion", inputs)).To(BeTrue())
	Expect(BoolInput("diff_mode", inputs)).To(BeFalse())
	Expect(BoolInput("missing", inputs)).To(BeFalse())

	Expect(ConfirmDestructive(inputs, "delete the inventory")).To(BeNil())
	Expect(ConfirmDestructive([]*core.Connection{}, "delete the inventory")).To(HaveOccurred())
}

// TestSetBoolIfSetPreservesOmit: AWX defaults a host's `enabled` to TRUE, and the
// manifest cannot carry a default Value — so an untouched checkbox must mean
// "let AWX decide", not "false", or adding a host would silently disable it.
func TestSetBoolIfSetPreservesOmit(t *testing.T) {
	RegisterTestingT(t)

	body := map[string]interface{}{}
	SetBoolIfSet(body, []*core.Connection{{Name: "enabled", Type: core.ConnectionTypeBoolean, Value: nil}}, "enabled", "enabled")
	Expect(body).NotTo(HaveKey("enabled"), "an untouched checkbox must be omitted, not sent as false")

	SetBoolIfSet(body, []*core.Connection{{Name: "enabled", Type: core.ConnectionTypeBoolean, Value: ""}}, "enabled", "enabled")
	Expect(body).NotTo(HaveKey("enabled"), "an unresolved ${var.x} substitutes to \"\" — that is untouched, not false")

	SetBoolIfSet(body, []*core.Connection{{Name: "enabled", Type: core.ConnectionTypeBoolean, Value: false}}, "enabled", "enabled")
	Expect(body["enabled"]).To(Equal(false))

	SetBoolIfSet(body, []*core.Connection{{Name: "enabled", Type: core.ConnectionTypeBoolean, Value: "true"}}, "enabled", "enabled")
	Expect(body["enabled"]).To(Equal(true))
}

// TestMergeAdditionalFieldsWinsLast pins the power-user precedence contract.
func TestMergeAdditionalFieldsWinsLast(t *testing.T) {
	RegisterTestingT(t)

	body := map[string]interface{}{"name": "from-input", "description": "kept"}
	inputs := []*core.Connection{
		{Name: "additional_fields", Type: core.ConnectionTypeObject, Value: `{"name":"from-additional","forks":4}`},
	}
	Expect(MergeAdditionalFields(body, inputs)).To(BeNil())
	Expect(body["name"]).To(Equal("from-additional"))
	Expect(body["description"]).To(Equal("kept"))
	Expect(body["forks"]).To(Equal(float64(4)))

	bad := []*core.Connection{{Name: "additional_fields", Type: core.ConnectionTypeObject, Value: `[1,2]`}}
	Expect(MergeAdditionalFields(map[string]interface{}{}, bad)).To(HaveOccurred())
}

func TestIDString(t *testing.T) {
	RegisterTestingT(t)

	// AWX ids are JSON NUMBERS — a naive v.(string) yields "".
	Expect(IDString(float64(7))).To(Equal("7"))
	Expect(IDString(int64(7))).To(Equal("7"))
	Expect(IDString(7)).To(Equal("7"))
	Expect(IDString("7")).To(Equal("7"))
	Expect(IDString(nil)).To(Equal(""))
	Expect(IDString(5.211)).To(Equal("5.211")) // elapsed seconds
}

// ---------------------------------------------------------------------------
// ★ Launch pre-flight
// ---------------------------------------------------------------------------

// ★ TestValidatePromptsRefusesIgnoredField is the safety property the whole node
// exists to provide. AWX answers 201 and SILENTLY DROPS any prompt field whose
// ask_*_on_launch is false — so sending limit=web* to a template that does not
// prompt for it RUNS THE PLAYBOOK AGAINST EVERY HOST IN THE INVENTORY, with the
// only trace being ignored_fields in the response body.
func TestValidatePromptsRefusesIgnoredField(t *testing.T) {
	RegisterTestingT(t)

	cfg := LaunchConfig{Ask: map[string]bool{"limit": false, "extra_vars": false, "job_tags": false, "inventory": true}}

	err := ValidatePrompts(cfg, map[string]interface{}{"limit": "web*"}, nil, false)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Limit"))
	Expect(err.Error()).To(ContainSubstring("every host"))
	Expect(err.Error()).To(ContainSubstring("Prompt on launch"))

	// The operator's explicit escape hatch.
	Expect(ValidatePrompts(cfg, map[string]interface{}{"limit": "web*"}, nil, true)).To(BeNil())

	// A field the template DOES prompt for is fine.
	Expect(ValidatePrompts(cfg, map[string]interface{}{"inventory": int64(1)}, nil, false)).To(BeNil())

	// A field with no ask_* flag at all (credential_passwords) is AWX's business.
	Expect(ValidatePrompts(cfg, map[string]interface{}{"credential_passwords": map[string]interface{}{"x": "y"}}, nil, false)).To(BeNil())

	// ★ Survey answers BYPASS ask_variables_on_launch — never gate them on it.
	surveyVars := map[string]bool{"target_env": true}
	body := map[string]interface{}{"extra_vars": map[string]interface{}{"target_env": "prod"}}
	Expect(ValidatePrompts(cfg, body, surveyVars, false)).To(BeNil())

	// …but a NON-survey extra variable still needs ask_variables_on_launch.
	body = map[string]interface{}{"extra_vars": map[string]interface{}{"target_env": "prod", "rogue": 1}}
	err = ValidatePrompts(cfg, body, surveyVars, false)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("rogue"))
	Expect(err.Error()).NotTo(ContainSubstring("target_env"))

	// The three irregular ask_* → field mappings are wired up.
	Expect(askFieldMap["ask_variables_on_launch"]).To(Equal("extra_vars"))
	Expect(askFieldMap["ask_tags_on_launch"]).To(Equal("job_tags"))
	Expect(askFieldMap["ask_credential_on_launch"]).To(Equal("credentials"))
	Expect(askFieldMap).To(HaveLen(16))
}

// TestValidatePromptsEnforcesPasswordsAndInventory covers the two pre-conditions
// that are hard AWX 400s rather than silent drops.
func TestValidatePromptsEnforcesPasswordsAndInventory(t *testing.T) {
	RegisterTestingT(t)

	cfg := LaunchConfig{Ask: map[string]bool{}, PasswordsNeededToStart: []string{"ssh_password", "vault_password.dev"}}
	err := ValidatePrompts(cfg, map[string]interface{}{}, nil, false)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("ssh_password"))
	Expect(err.Error()).To(ContainSubstring("vault_password.dev"))

	ok := map[string]interface{}{"credential_passwords": map[string]interface{}{"ssh_password": "x", "vault_password.dev": "y"}}
	Expect(ValidatePrompts(cfg, ok, nil, false)).To(BeNil())

	// An unlaunchable template: no inventory of its own, and it will not prompt.
	unlaunchable := LaunchConfig{Ask: map[string]bool{"inventory": false}, InventoryNeededToStart: true}
	err = ValidatePrompts(unlaunchable, map[string]interface{}{}, nil, false)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("cannot be launched"))

	// It prompts for one, but the operator did not choose one.
	needsChoice := LaunchConfig{Ask: map[string]bool{"inventory": true}, InventoryNeededToStart: true}
	err = ValidatePrompts(needsChoice, map[string]interface{}{}, nil, false)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("must choose one"))
	Expect(ValidatePrompts(needsChoice, map[string]interface{}{"inventory": int64(1)}, nil, false)).To(BeNil())
}

// TestPreflightAndSurveyAgainstAWXShapes drives the real bodies the live AWX
// 24.6.1 instance returns for Demo Job Template (id 7), which carries three
// required survey questions — two of them multiselect.
func TestPreflightAndSurveyAgainstAWXShapes(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/job_templates/7/launch/":
			_, _ = w.Write([]byte(`{
				"can_start_without_user_input": false,
				"passwords_needed_to_start": [],
				"ask_variables_on_launch": false,
				"ask_limit_on_launch": false,
				"ask_tags_on_launch": true,
				"variables_needed_to_start": ["stopandrebuilt","target_hosts","target_group"],
				"survey_enabled": true,
				"inventory_needed_to_start": false,
				"defaults": {"limit": "", "job_tags": ""}
			}`))
		case "/api/v2/job_templates/7/survey_spec/":
			_, _ = w.Write([]byte(`{"name":"","description":"","spec":[
				{"question_name":"Stop and rebuild?","variable":"stopandrebuilt","type":"multiplechoice","required":true,"default":"","choices":["true","false"]},
				{"question_name":"Target hosts","variable":"target_hosts","type":"multiselect","required":true,"default":"","choices":["none","osmp-01","osmp-02"]},
				{"question_name":"Forks","variable":"forks_wanted","type":"integer","required":false,"default":null,"min":1,"max":10}
			]}`))
		case "/api/v2/job_templates/8/survey_spec/":
			_, _ = w.Write([]byte(`{}`)) // NO SURVEY: HTTP 200 with an EMPTY OBJECT, not a 404
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	a := tokenAuth(srv.URL)
	ctx := context.Background()

	cfg, err := PreflightLaunch(ctx, a, TemplateKindJob, 7)
	Expect(err).To(BeNil())
	Expect(cfg.SurveyEnabled).To(BeTrue())
	Expect(cfg.CanStartWithoutUserInput).To(BeFalse())
	Expect(cfg.Ask["limit"]).To(BeFalse())
	Expect(cfg.Ask["job_tags"]).To(BeTrue(), "ask_tags_on_launch gates job_tags, not tags")
	Expect(cfg.Ask["extra_vars"]).To(BeFalse(), "ask_variables_on_launch gates extra_vars")
	Expect(cfg.VariablesNeededToStart).To(ConsistOf("stopandrebuilt", "target_hosts", "target_group"))
	Expect(cfg.PromptableFields()).To(ConsistOf("job_tags"))
	Expect(cfg.Defaults).To(HaveKey("limit"))

	spec, err := FetchSurveySpec(ctx, a, TemplateKindJob, 7)
	Expect(err).To(BeNil())
	Expect(spec.HasSurvey()).To(BeTrue())
	Expect(spec.Spec).To(HaveLen(3))
	Expect(spec.VariableNames()).To(HaveKey("target_hosts"))
	Expect(spec.RequiredVariables()).To(ConsistOf("stopandrebuilt", "target_hosts"))
	Expect(spec.Spec[0].Choices).To(Equal([]string{"true", "false"}))
	Expect(*spec.Spec[2].Min).To(Equal(float64(1)))

	// A template with no survey answers 200 {} — not a 404.
	empty, err := FetchSurveySpec(ctx, a, TemplateKindJob, 8)
	Expect(err).To(BeNil())
	Expect(empty.HasSurvey()).To(BeFalse())

	// Required answers missing.
	err = ValidateSurvey(spec, map[string]interface{}{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("stopandrebuilt"))
	Expect(err.Error()).To(ContainSubstring("target_hosts"))
	Expect(err.Error()).NotTo(ContainSubstring("forks_wanted"), "an optional question is not required")

	// A multiselect answer is a JSON ARRAY; a multiplechoice one is a scalar.
	answers := map[string]interface{}{
		"stopandrebuilt": "false",
		"target_hosts":   []interface{}{"none"},
	}
	Expect(ValidateSurvey(spec, answers)).To(BeNil())

	// Off-list choices, wrong shapes and out-of-range numbers are caught here
	// rather than as an opaque AWX 400.
	err = ValidateSurvey(spec, map[string]interface{}{"stopandrebuilt": "maybe", "target_hosts": []interface{}{"none"}})
	Expect(err.Error()).To(ContainSubstring("must be one of"))

	err = ValidateSurvey(spec, map[string]interface{}{"stopandrebuilt": "true", "target_hosts": "none"})
	Expect(err.Error()).To(ContainSubstring("must be a list"))

	err = ValidateSurvey(spec, map[string]interface{}{
		"stopandrebuilt": "true", "target_hosts": []interface{}{"none"}, "forks_wanted": 50,
	})
	Expect(err.Error()).To(ContainSubstring("at most 10"))

	// ValidateLaunch composes the lot: survey + the ignored-fields refusal.
	_, err = ValidateLaunch(ctx, a, TemplateKindJob, 7, map[string]interface{}{
		"extra_vars": answers,
		"limit":      "web*", // ask_limit_on_launch is FALSE
	}, false)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Limit"))

	// Survey answers alone launch cleanly, even though ask_variables_on_launch is off.
	_, err = ValidateLaunch(ctx, a, TemplateKindJob, 7, map[string]interface{}{"extra_vars": answers}, false)
	Expect(err).To(BeNil())
}

// ★ A DEFAULT DOES NOT EXCUSE A REQUIRED SURVEY QUESTION.
//
// It is the obvious assumption — AWX is holding a default, so surely it fills it
// in — and it is wrong. AWX validates the extra_vars you SUBMITTED
// (SurveyJobTemplateMixin._survey_element_validation: `if variable not in data and
// required -> "'x' value missing"`) and only applies the survey's defaults
// afterwards, when it builds the job.
//
// Verified against AWX 24.6.1, job template 7, whose target_hosts and target_group
// are REQUIRED multiselects with default "none":
//
//	POST job_templates/7/launch/ {"extra_vars":{"stopandrebuilt":"false"}}
//	-> 400 {"variables_needed_to_start":["'target_hosts' value missing",
//	                                     "'target_group' value missing"]}
//
// If this exemption ever creeps back, the client-side pre-flight passes a body AWX
// then rejects — which defeats the entire point of having one.
func TestRequiredSurveyQuestionIsNotExcusedByItsDefault(t *testing.T) {
	RegisterTestingT(t)

	spec := SurveySpec{Spec: []SurveyQuestion{
		{Variable: "stopandrebuilt", QuestionName: "Stop and rebuild", Type: "multiplechoice",
			Required: true, Default: "", Choices: []string{"true", "false"}},
		{Variable: "target_hosts", QuestionName: "Inventory to target", Type: "multiselect",
			Required: true, Default: "none", Choices: []string{"none", "osmp-01"}},
		{Variable: "optional_forks", QuestionName: "Forks", Type: "integer",
			Required: false, Default: float64(5)},
	}}

	// It must agree with AWX's own variables_needed_to_start, which lists EVERY
	// required question — default or no default.
	Expect(spec.RequiredVariables()).To(Equal([]string{"stopandrebuilt", "target_hosts"}))

	// Answering only the defaultless one is exactly the 400 above.
	err := ValidateSurvey(spec, map[string]interface{}{"stopandrebuilt": "false"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("target_hosts"))
	Expect(err.Error()).ToNot(ContainSubstring("optional_forks"), "an OPTIONAL question is still exempt")

	// ★ And the message shows the default in the ARRAY shape a multiselect answer
	// must take — AWX stores it as the scalar "none", but ["none"] is what it will
	// accept, so echoing the default verbatim would hand back a value AWX rejects.
	Expect(err.Error()).To(ContainSubstring(`["none"]`))

	// Sending it explicitly is what AWX wants, and it validates.
	Expect(ValidateSurvey(spec, map[string]interface{}{
		"stopandrebuilt": "false",
		"target_hosts":   []interface{}{"none"},
	})).To(BeNil())
}

// A rejected choice names the VALUE that was rejected, not merely the allowed
// list: a survey with a dozen options otherwise leaves the operator diffing two
// lists by eye to find their own typo.
func TestSurveyChoiceErrorNamesTheRejectedValue(t *testing.T) {
	RegisterTestingT(t)

	spec := SurveySpec{Spec: []SurveyQuestion{
		{Variable: "target_group", Type: "multiselect", Required: true,
			Choices: []string{"none", "Corsham", "IICS"}},
		{Variable: "mode", Type: "multiplechoice", Required: true,
			Choices: []string{"run", "check"}},
	}}

	err := ValidateSurvey(spec, map[string]interface{}{
		"target_group": []interface{}{"Atlantis"},
		"mode":         "run",
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Atlantis"))
	Expect(err.Error()).To(ContainSubstring("may only contain: none, Corsham, IICS"))

	err = ValidateSurvey(spec, map[string]interface{}{
		"target_group": []interface{}{"none"},
		"mode":         "sideways",
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("sideways"))
	Expect(err.Error()).To(ContainSubstring("must be one of: run, check"))
}

// TestCheckIgnoredFields is the belt-and-braces half of the guard: a template can
// be reconfigured between the pre-flight and the launch.
func TestCheckIgnoredFields(t *testing.T) {
	RegisterTestingT(t)

	launched := map[string]interface{}{"id": float64(9), "ignored_fields": map[string]interface{}{"limit": "web*"}}

	ignored, err := CheckIgnoredFields(launched, false)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("IGNORED"))
	Expect(err.Error()).To(ContainSubstring("Limit"))
	Expect(ignored).To(HaveKey("limit"), "ignored_fields is emitted either way")

	ignored, err = CheckIgnoredFields(launched, true)
	Expect(err).To(BeNil())
	Expect(ignored).To(HaveKey("limit"))

	ignored, err = CheckIgnoredFields(map[string]interface{}{"ignored_fields": map[string]interface{}{}}, false)
	Expect(err).To(BeNil())
	Expect(ignored).To(BeEmpty())
}

// ---------------------------------------------------------------------------
// Waiting
// ---------------------------------------------------------------------------

// TestWaitForJobPollsTheListThenReadsTheDetail pins three properties at once: the
// hot loop hits the cheap LIST endpoint (the detail view runs two COUNT(*)
// queries over the job-events table on EVERY call), the terminal test is a
// non-null `finished` timestamp, and stdout is only read once
// event_processing_finished is true — AWX writes job events asynchronously, so
// reading them the instant the status flips yields truncated or empty output.
func TestWaitForJobPollsTheListThenReadsTheDetail(t *testing.T) {
	RegisterTestingT(t)

	var listCalls, detailCalls int32
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/jobs/" && r.URL.Query().Get("id") == "4":
			n := atomic.AddInt32(&listCalls, 1)
			if n == 1 {
				_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":4,"status":"running","finished":null}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":4,"status":"successful","finished":"2026-07-14T10:00:05Z"}]}`))
		case r.URL.Path == "/api/v2/jobs/4/":
			n := atomic.AddInt32(&detailCalls, 1)
			settled := "false"
			if n > 1 {
				settled = "true"
			}
			_, _ = fmt.Fprintf(w, `{"id":4,"type":"job","status":"successful","failed":false,
				"finished":"2026-07-14T10:00:05Z","elapsed":5.211,"event_processing_finished":%s,
				"artifacts":{"build":"42"},"host_status_counts":{"ok":1},"result_traceback":""}`, settled)
		case r.URL.Path == "/api/v2/jobs/4/stdout/":
			_, _ = w.Write([]byte("PLAY RECAP *** localhost : ok=1"))
		default:
			w.WriteHeader(http.StatusNotFound)
			t.Errorf("unexpected request %s", r.URL)
		}
	})
	defer srv.Close()

	a := tokenAuth(srv.URL)
	res, err := WaitForJob(context.Background(), a, JobKindJob, 4, WaitOpts{
		PollIntervalSeconds: 1, TimeoutSeconds: 30, IncludeStdout: true,
	})
	Expect(err).To(BeNil())
	Expect(res.TimedOut).To(BeFalse())
	Expect(res.EventsSettled).To(BeTrue())
	Expect(res.Stdout).To(ContainSubstring("PLAY RECAP"))
	Expect(res.StdoutTruncated).To(BeFalse())

	Expect(atomic.LoadInt32(&listCalls)).To(Equal(int32(2)), "the hot loop must poll the cheap LIST endpoint")
	Expect(atomic.LoadInt32(&detailCalls)).To(BeNumerically(">=", 2), "one detail GET for the terminal record, then again until the events settle")

	out := JobOutputs(a, JobKindJob, res.Job)
	Expect(out["job_id"]).To(Equal("4"))
	Expect(out["status"]).To(Equal("successful"))
	Expect(out["finished"]).To(Equal(true))
	Expect(out["failed"]).To(Equal(false))
	Expect(out["elapsed"]).To(Equal("5.211"))
	Expect(out["artifacts"]).To(Equal(map[string]interface{}{"build": "42"}))
	Expect(out["job_url"]).To(Equal(srv.URL + "/#/jobs/playbook/4/output"))
}

// TestWaitForJobTimesOutWithoutCancelling: a wait that runs out must NOT silently
// kill a production job — that is surprising and destructive — unless the
// operator asked for it.
func TestWaitForJobTimesOutWithoutCancelling(t *testing.T) {
	RegisterTestingT(t)

	var cancels int32
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/jobs/":
			_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":4,"status":"running","finished":null}]}`))
		case "/api/v2/jobs/4/":
			_, _ = w.Write([]byte(`{"id":4,"status":"running","finished":null,"job_explanation":"waiting for capacity"}`))
		case "/api/v2/jobs/4/cancel/":
			atomic.AddInt32(&cancels, 1)
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	a := tokenAuth(srv.URL)
	res, err := WaitForJob(context.Background(), a, JobKindJob, 4, WaitOpts{PollIntervalSeconds: 1, TimeoutSeconds: 1})
	Expect(err).To(BeNil(), "a timeout is a SOFT failure with the last-seen status, not a Go error")
	Expect(res.TimedOut).To(BeTrue())
	Expect(res.Canceled).To(BeFalse())
	Expect(atomic.LoadInt32(&cancels)).To(Equal(int32(0)), "never auto-cancel unless cancel_on_timeout is ticked")
	Expect(StringField(res.Job, "status")).To(Equal("running"))
	Expect(StringField(res.Job, "job_explanation")).To(Equal("waiting for capacity"))

	res, err = WaitForJob(context.Background(), a, JobKindJob, 4, WaitOpts{PollIntervalSeconds: 1, TimeoutSeconds: 1, CancelOnTimeout: true})
	Expect(err).To(BeNil())
	Expect(res.TimedOut).To(BeTrue())
	Expect(res.Canceled).To(BeTrue())
	Expect(atomic.LoadInt32(&cancels)).To(Equal(int32(1)))
}

// ★ TestWaitForJobDoesNotInheritTheCallersDeadline. Every context in an action is
// manufactured by the action itself, and the obvious thing to reach for is the
// ordinary 75-second awx.Context(). If WaitForJob inherited that, a 10-minute wait
// would be cut short after 75 seconds and a perfectly healthy job reported as
// timed out — the same bug replicated across all nine waiting actions. The wait is
// bounded by opts.TimeoutSeconds and nothing else.
func TestWaitForJobDoesNotInheritTheCallersDeadline(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/jobs/":
			_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":4,"status":"successful","finished":"2026-07-14T10:00:05Z"}]}`))
		case "/api/v2/jobs/4/":
			_, _ = w.Write([]byte(`{"id":4,"status":"successful","finished":"2026-07-14T10:00:05Z","event_processing_finished":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	// A context that is ALREADY CANCELLED. A wait that honoured it would fail
	// instantly; this one is governed by its own opts.
	dead, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := WaitForJob(dead, tokenAuth(srv.URL), JobKindJob, 4, WaitOpts{PollIntervalSeconds: 1, TimeoutSeconds: 30})
	Expect(err).To(BeNil())
	Expect(res.TimedOut).To(BeFalse())
	Expect(StringField(res.Job, "status")).To(Equal("successful"))
}

// TestCancelJobTreats405AsAlreadyFinished: POSTing /cancel/ on a finished job
// answers 405 Method Not Allowed — not 409, not 400. Treating it as a routing
// error would report a perfectly successful no-op as a failure.
func TestCancelJobTreats405AsAlreadyFinished(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/jobs/4/cancel/":
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte(`{"detail":"Method \"POST\" not allowed."}`))
		case "/api/v2/jobs/5/cancel/":
			w.WriteHeader(http.StatusAccepted) // 202, with a COMPLETELY EMPTY body
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	a := tokenAuth(srv.URL)

	already, err := CancelJob(context.Background(), a, JobKindJob, 4)
	Expect(err).To(BeNil())
	Expect(already).To(BeTrue())

	already, err = CancelJob(context.Background(), a, JobKindJob, 5)
	Expect(err).To(BeNil())
	Expect(already).To(BeFalse())
}

func TestJobKindPaths(t *testing.T) {
	RegisterTestingT(t)

	Expect(JobKindPaths).To(HaveLen(5))
	for kind, want := range map[string]string{
		JobKindJob: "jobs/", JobKindWorkflowJob: "workflow_jobs/", JobKindAdHocCommand: "ad_hoc_commands/",
		JobKindProjectUpdate: "project_updates/", JobKindInventoryUpdate: "inventory_updates/",
	} {
		got, err := JobKindPath(kind)
		Expect(err).To(BeNil())
		Expect(got).To(Equal(want))
	}

	got, err := JobKindPath("") // a blank job_kind means a plain job
	Expect(err).To(BeNil())
	Expect(got).To(Equal("jobs/"))

	_, err = JobKindPath("nonsense")
	Expect(err).To(HaveOccurred())

	Expect(ClampWaitSeconds(0)).To(Equal(DefaultWaitSeconds))
	Expect(ClampWaitSeconds(120)).To(Equal(120))
	Expect(ClampWaitSeconds(99999)).To(Equal(MaxWaitSeconds), "a waiting node pins a flow worker for its whole duration")
}

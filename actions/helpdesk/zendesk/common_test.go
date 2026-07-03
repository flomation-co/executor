package zendesk

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func strConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func secretConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeSecret, Value: value}
}

func TestNormaliseSubdomain(t *testing.T) {
	RegisterTestingT(t)
	for input, want := range map[string]string{
		"mycompany":                                   "mycompany",
		"  mycompany  ":                               "mycompany",
		"mycompany.zendesk.com":                       "mycompany",
		"https://mycompany.zendesk.com":               "mycompany",
		"https://mycompany.zendesk.com/":              "mycompany",
		"https://mycompany.zendesk.com/agent/tickets": "mycompany",
		"http://mycompany.zendesk.com/agent":          "mycompany",
		// Host-injection attempt is stripped to the bare handle.
		"acme.evil.com/x": "acme",
	} {
		Expect(NormaliseSubdomain(input)).To(Equal(want), "input: %q", input)
	}
}

func TestBuildURL(t *testing.T) {
	RegisterTestingT(t)
	Expect(BuildURL("acme", "/tickets.json")).To(Equal("https://acme.zendesk.com/api/v2/tickets.json"))
}

func TestGetAuth_APIToken(t *testing.T) {
	RegisterTestingT(t)
	sub, auth, err := GetAuth([]*core.Connection{
		strConn("subdomain", "acme"),
		strConn("email", "agent@acme.com"),
		secretConn("api_token", "tok123"),
	})
	Expect(err).To(BeNil())
	Expect(sub).To(Equal("acme"))
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("agent@acme.com/token:tok123"))
	Expect(auth).To(Equal(want))
}

func TestGetAuth_OAuthBearerWins(t *testing.T) {
	RegisterTestingT(t)
	_, auth, err := GetAuth([]*core.Connection{
		strConn("subdomain", "acme"),
		strConn("email", "agent@acme.com"),
		secretConn("api_token", "tok123"),
		secretConn("oauth_token", "bearer-abc"),
	})
	Expect(err).To(BeNil())
	Expect(auth).To(Equal("Bearer bearer-abc"))
}

func TestGetAuth_Errors(t *testing.T) {
	RegisterTestingT(t)
	// Missing subdomain.
	_, _, err := GetAuth([]*core.Connection{strConn("email", "a@b.com"), secretConn("api_token", "t")})
	Expect(err).NotTo(BeNil())
	// Bad subdomain charset that survives normalisation (underscore).
	_, _, err = GetAuth([]*core.Connection{strConn("subdomain", "acme_corp"), strConn("email", "a@b.com"), secretConn("api_token", "t")})
	Expect(err).NotTo(BeNil())
	// No credentials at all.
	_, _, err = GetAuth([]*core.Connection{strConn("subdomain", "acme")})
	Expect(err).NotTo(BeNil())
}

func TestCheckResponse(t *testing.T) {
	RegisterTestingT(t)
	Expect(CheckResponse(&APIResponse{StatusCode: 200})).To(BeNil())

	// String error + description.
	body := []byte(`{"error":"RecordNotFound","description":"Not found"}`)
	err := CheckResponse(&APIResponse{StatusCode: 404, Body: body})
	Expect(err).NotTo(BeNil())
	Expect(err.Error()).To(ContainSubstring("RecordNotFound"))
	Expect(err.Error()).To(ContainSubstring("Not found"))

	// Object error with title/message.
	body = []byte(`{"error":{"title":"Invalid","message":"bad field"}}`)
	err = CheckResponse(&APIResponse{StatusCode: 400, Body: body})
	Expect(err.Error()).To(ContainSubstring("Invalid"))
	Expect(err.Error()).To(ContainSubstring("bad field"))

	// 429 with Retry-After.
	h := http.Header{}
	h.Set("Retry-After", "30")
	err = CheckResponse(&APIResponse{StatusCode: 429, Headers: h})
	Expect(err.Error()).To(ContainSubstring("30"))
}

func TestSplitCSV(t *testing.T) {
	RegisterTestingT(t)
	Expect(SplitCSV("")).To(BeNil())
	Expect(SplitCSV("  ")).To(BeNil())
	Expect(SplitCSV("vip, wholesale ,, ")).To(Equal([]string{"vip", "wholesale"}))
}

func TestSetNumericIDIfPresent(t *testing.T) {
	RegisterTestingT(t)
	body := map[string]interface{}{}
	SetNumericIDIfPresent(body, []*core.Connection{strConn("group_id", "360001")}, "group_id", "group_id")
	Expect(body["group_id"]).To(Equal(int64(360001)))
	// Non-numeric falls through as a string rather than being dropped.
	body = map[string]interface{}{}
	SetNumericIDIfPresent(body, []*core.Connection{strConn("group_id", "abc")}, "group_id", "group_id")
	Expect(body["group_id"]).To(Equal("abc"))
	// Empty is omitted.
	body = map[string]interface{}{}
	SetNumericIDIfPresent(body, []*core.Connection{strConn("group_id", "")}, "group_id", "group_id")
	_, ok := body["group_id"]
	Expect(ok).To(BeFalse())
}

func TestStringifyID(t *testing.T) {
	RegisterTestingT(t)
	Expect(StringifyID(float64(35436))).To(Equal("35436"))
	Expect(StringifyID("abc")).To(Equal("abc"))
	Expect(StringifyID(nil)).To(Equal(""))
}

// TestListResources_FollowsNextPage exercises the return-all pagination loop
// against an httptest server that hands back an absolute next_page URL, the way
// Zendesk does.
func TestListResources_FollowsNextPage(t *testing.T) {
	RegisterTestingT(t)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assert auth header is forwarded on every page.
		Expect(r.Header.Get("Authorization")).To(HavePrefix("Basic "))
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		if page == "2" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"users":     []map[string]interface{}{{"id": 3}},
				"next_page": nil,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"users":     []map[string]interface{}{{"id": 1}, {"id": 2}},
			"next_page": srv.URL + "/api/v2/users.json?page=2",
		})
	}))
	defer srv.Close()
	restore := SetHostForTest(srv.URL)
	defer restore()

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("a@b.com/token:t"))
	items, next, _, pages, err := ListResources("acme", auth, "/users.json", "users", url.Values{}, true)
	Expect(err).To(BeNil())
	Expect(items).To(HaveLen(3))
	Expect(next).To(Equal(""))
	Expect(pages).To(Equal(2))
}

// TestListResources_SinglePage confirms a non-return-all call fetches one page
// and surfaces the outstanding next_page cursor for manual resume.
func TestListResources_SinglePage(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Query().Get("per_page")).To(Equal("2"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"users":[{"id":1},{"id":2}],"next_page":"https://acme.zendesk.com/api/v2/users.json?page=2"}`)
	}))
	defer srv.Close()
	restore := SetHostForTest(srv.URL)
	defer restore()

	q := url.Values{}
	q.Set("per_page", "2")
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("a@b.com/token:t"))
	items, next, _, pages, err := ListResources("acme", auth, "/users.json", "users", q, false)
	Expect(err).To(BeNil())
	Expect(items).To(HaveLen(2))
	Expect(pages).To(Equal(1))
	Expect(strings.Contains(next, "page=2")).To(BeTrue())
}

// TestCreateResource wraps/unwraps the single-key envelope.
func TestCreateResource(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPost))
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		ticket, _ := payload["ticket"].(map[string]interface{})
		Expect(ticket["subject"]).To(Equal("Hi"))
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"ticket":{"id":99,"subject":"Hi"}}`)
	}))
	defer srv.Close()
	restore := SetHostForTest(srv.URL)
	defer restore()

	auth := "Bearer x"
	resp, err := CreateResource("acme", auth, "/tickets.json", "ticket", map[string]interface{}{"subject": "Hi"})
	Expect(err).To(BeNil())
	out := ResourceResult(resp, "ticket", "done")
	Expect(out["id"]).To(Equal("99"))
	Expect(out["success"]).To(Equal(true))
}

package wordpress

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestNormaliseBaseURL(t *testing.T) {
	RegisterTestingT(t)
	for input, want := range map[string]string{
		"site.com":                       "https://site.com",
		"  site.com/  ":                  "https://site.com",
		"https://site.com":               "https://site.com",
		"https://site.com/":              "https://site.com",
		"http://site.com":                "http://site.com",
		"https://site.com/blog":          "https://site.com/blog",
		"https://site.com/wp-json/wp/v2": "https://site.com",
		"https://site.com/wp-json":       "https://site.com",
	} {
		got, err := NormaliseBaseURL(input)
		Expect(err).To(BeNil(), "input: %q", input)
		Expect(got).To(Equal(want), "input: %q", input)
	}
	for _, bad := range []string{"", "  ", "ftp://site.com", "https://"} {
		_, err := NormaliseBaseURL(bad)
		Expect(err).To(HaveOccurred(), "input: %q", bad)
	}
}

func TestGetAuth(t *testing.T) {
	RegisterTestingT(t)
	a, err := GetAuth([]*core.Connection{
		{Name: "url", Type: core.ConnectionTypeString, Value: "site.com"},
		{Name: "username", Type: core.ConnectionTypeString, Value: "admin"},
		{Name: "app_password", Type: core.ConnectionTypeSecret, Value: "abcd efgh"},
		{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Value: true},
	})
	Expect(err).To(BeNil())
	Expect(a.BaseURL).To(Equal("https://site.com"))
	Expect(a.Username).To(Equal("admin"))
	Expect(a.Password).To(Equal("abcd efgh"))
	Expect(a.Insecure).To(BeTrue())

	for _, missing := range []string{"url", "username", "app_password"} {
		in := []*core.Connection{
			{Name: "url", Type: core.ConnectionTypeString, Value: "site.com"},
			{Name: "username", Type: core.ConnectionTypeString, Value: "admin"},
			{Name: "app_password", Type: core.ConnectionTypeSecret, Value: "pw"},
		}
		filtered := in[:0]
		for _, c := range in {
			if c.Name != missing {
				filtered = append(filtered, c)
			}
		}
		_, err := GetAuth(filtered)
		Expect(err).To(HaveOccurred(), "missing: %q", missing)
	}
}

func TestExecuteAPISendsBasicAuth(t *testing.T) {
	RegisterTestingT(t)
	var gotUser, gotPass, gotPath string
	var hadBasic bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, hadBasic = r.BasicAuth()
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()
	defer SetBaseForTest(server.URL)()

	a := Auth{BaseURL: "https://ignored", Username: "admin", Password: "pw"}
	resp, err := ExecuteAPI(a, http.MethodGet, "/posts/1", nil)
	Expect(err).To(BeNil())
	Expect(resp.StatusCode).To(Equal(200))
	Expect(hadBasic).To(BeTrue())
	Expect(gotUser).To(Equal("admin"))
	Expect(gotPass).To(Equal("pw"))
	Expect(gotPath).To(Equal("/posts/1"))
}

func TestUpdateUsesPost(t *testing.T) {
	RegisterTestingT(t)
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":9,"title":"x"}`))
	}))
	defer server.Close()
	defer SetBaseForTest(server.URL)()

	out, err := UpdateResource(Auth{}, "/posts/9", map[string]interface{}{"title": "x"})
	Expect(err).To(BeNil())
	Expect(gotMethod).To(Equal(http.MethodPost)) // WordPress updates via POST, not PUT
	Expect(out["id"]).To(Equal(float64(9)))
}

func TestCheckResponse(t *testing.T) {
	RegisterTestingT(t)
	Expect(CheckResponse(&APIResponse{StatusCode: 200})).To(BeNil())
	err := CheckResponse(&APIResponse{StatusCode: 404, Body: []byte(`{"code":"rest_post_invalid_id","message":"Invalid post ID.","data":{"status":404}}`)})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("WordPress API error (404)"))
	Expect(err.Error()).To(ContainSubstring("Invalid post ID."))
	Expect(err.Error()).To(ContainSubstring("rest_post_invalid_id"))
}

func TestListResourcesReturnAllPaginates(t *testing.T) {
	RegisterTestingT(t)
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		w.Header().Set("Content-Type", "application/json")
		switch call {
		case 1:
			Expect(r.URL.Query().Get("page")).To(Equal("1"))
			Expect(r.URL.Query().Get("search")).To(Equal("hello"))
			w.Header().Set("X-WP-Total", "2")
			w.Header().Set("X-WP-TotalPages", "2")
			w.Header().Set("Link", `<https://x/wp-json/wp/v2/posts?page=2>; rel="next"`)
			_, _ = w.Write([]byte(`[{"id":1}]`))
		case 2:
			Expect(r.URL.Query().Get("page")).To(Equal("2"))
			_, _ = w.Write([]byte(`[{"id":2}]`))
		default:
			t.Fatalf("unexpected extra page fetch: call %d", call)
		}
	}))
	defer server.Close()
	defer SetBaseForTest(server.URL)()

	q := url.Values{"search": {"hello"}, "per_page": {"1"}}
	items, total, totalPages, pages, err := ListResources(Auth{}, "/posts", q, true)
	Expect(err).To(BeNil())
	Expect(pages).To(Equal(2))
	Expect(total).To(Equal(2))
	Expect(totalPages).To(Equal(2))
	Expect(len(items)).To(Equal(2))
}

func TestResourceResultUnwrapsForceDeletePrevious(t *testing.T) {
	RegisterTestingT(t)
	// A permanent (force) delete returns {deleted, previous:{id}} with no
	// top-level id — the id must be unwrapped from previous.
	out := ResourceResult(map[string]interface{}{
		"deleted":  true,
		"previous": map[string]interface{}{"id": float64(42), "name": "x"},
	}, "deleted")
	Expect(out["id"]).To(Equal("42"))
	// A trash delete (bare object with id) is unaffected.
	out = ResourceResult(map[string]interface{}{"id": float64(7)}, "trashed")
	Expect(out["id"]).To(Equal("7"))
}

func TestListResourcesReturnAllUsesTotalPagesWhenLinkStripped(t *testing.T) {
	RegisterTestingT(t)
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if call > 3 {
			t.Fatalf("fetched more pages than X-WP-TotalPages")
		}
		w.Header().Set("Content-Type", "application/json")
		// NO Link header — pagination must continue off X-WP-TotalPages alone.
		w.Header().Set("X-WP-Total", "3")
		w.Header().Set("X-WP-TotalPages", "3")
		_, _ = w.Write([]byte(`[{"id":1}]`))
	}))
	defer server.Close()
	defer SetBaseForTest(server.URL)()

	items, _, totalPages, pages, err := ListResources(Auth{}, "/posts", url.Values{"per_page": {"1"}}, true)
	Expect(err).To(BeNil())
	Expect(totalPages).To(Equal(3))
	Expect(pages).To(Equal(3)) // continued to page 3 despite the missing Link header
	Expect(len(items)).To(Equal(3))
}

func TestDeleteResourceForceAndReassign(t *testing.T) {
	RegisterTestingT(t)
	var gotForce, gotReassign, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotForce = r.URL.Query().Get("force")
		gotReassign = r.URL.Query().Get("reassign")
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deleted":true}`))
	}))
	defer server.Close()
	defer SetBaseForTest(server.URL)()

	// User-style delete: force + reassign carried through extra.
	_, err := DeleteResource(Auth{}, "/users/5", true, url.Values{"reassign": {"1"}})
	Expect(err).To(BeNil())
	Expect(gotMethod).To(Equal(http.MethodDelete))
	Expect(gotForce).To(Equal("true"))
	Expect(gotReassign).To(Equal("1"))

	// Post-style trash: force omitted.
	_, err = DeleteResource(Auth{}, "/posts/5", false, nil)
	Expect(err).To(BeNil())
	Expect(gotForce).To(Equal(""))
}

func TestSetIntListIfPresent(t *testing.T) {
	RegisterTestingT(t)
	body := map[string]interface{}{}
	SetIntListIfPresent(body, []*core.Connection{
		{Name: "cats", Type: core.ConnectionTypeString, Value: "3, 7 ,x,"},
	}, "categories", "cats")
	Expect(body["categories"]).To(Equal([]interface{}{3, 7}))

	body = map[string]interface{}{}
	SetIntListIfPresent(body, []*core.Connection{{Name: "c", Type: core.ConnectionTypeString, Value: " , "}}, "categories", "c")
	_, ok := body["categories"]
	Expect(ok).To(BeFalse())
}

func TestRedactAuth(t *testing.T) {
	RegisterTestingT(t)
	a := Auth{Password: "supersecretpw"}
	Expect(redactAuth(a, "boom supersecretpw here")).To(Equal("boom REDACTED here"))
	Expect(redactAuth(Auth{}, "plain")).To(Equal("plain"))
}

func TestClampLimitAndResultShaping(t *testing.T) {
	RegisterTestingT(t)
	Expect(ClampLimit(0, false)).To(Equal(DefaultPageLimit))
	Expect(ClampLimit(500, true)).To(Equal(MaxPageLimit))
	Expect(ClampLimit(30, true)).To(Equal(30))

	out := ResourceResult(map[string]interface{}{"id": float64(55)}, "done")
	Expect(out["id"]).To(Equal("55"))
	Expect(out["success"]).To(Equal(true))

	list := ListResult(nil, 0, 0, "none")
	Expect(list["results"]).To(Equal([]interface{}{}))
	Expect(list["count"]).To(Equal(0))
}

func TestMergeAdditionalFieldsRejectsNonObject(t *testing.T) {
	RegisterTestingT(t)
	body := map[string]interface{}{}
	err := MergeAdditionalFields(body, []*core.Connection{
		{Name: "additional_fields", Type: core.ConnectionTypeObject, Value: `[1,2]`},
	})
	Expect(err).To(HaveOccurred())
	err = MergeAdditionalFields(body, []*core.Connection{
		{Name: "additional_fields", Type: core.ConnectionTypeObject, Value: `{"menu_order":1}`},
	})
	Expect(err).To(BeNil())
	Expect(body["menu_order"]).To(Equal(float64(1)))
}

func TestInsecureClientSelected(t *testing.T) {
	RegisterTestingT(t)
	Expect(clientFor(Auth{Insecure: true})).To(BeIdenticalTo(insecureHTTPClient))
	Expect(clientFor(Auth{Insecure: false})).To(BeIdenticalTo(httpClient))
}

package calendly

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestResourceURI(t *testing.T) {
	RegisterTestingT(t)
	Expect(ResourceURI("abc-123", "scheduled_events")).To(Equal("https://api.calendly.com/scheduled_events/abc-123"))
	Expect(ResourceURI(" https://api.calendly.com/event_types/xyz ", "event_types")).To(Equal("https://api.calendly.com/event_types/xyz"))
}

func TestExtractUUID(t *testing.T) {
	RegisterTestingT(t)
	Expect(ExtractUUID("abc-123")).To(Equal("abc-123"))
	Expect(ExtractUUID("https://api.calendly.com/scheduled_events/abc-123")).To(Equal("abc-123"))
	Expect(ExtractUUID("https://api.calendly.com/scheduled_events/abc-123/")).To(Equal("abc-123"))
}

func TestGetAuth(t *testing.T) {
	RegisterTestingT(t)
	_, err := GetAuth([]*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("authentication required"))

	token, err := GetAuth([]*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok_x"},
	})
	Expect(err).To(BeNil())
	Expect(token).To(Equal("tok_x"))
}

func TestCheckResponseErrorEnvelope(t *testing.T) {
	RegisterTestingT(t)
	err := CheckResponse(&APIResponse{
		StatusCode: 400,
		Body:       []byte(`{"title":"Invalid Argument","message":"The supplied parameters are invalid.","details":[{"parameter":"scope","message":"invalid scope"}]}`),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("The supplied parameters are invalid"))
	Expect(err.Error()).To(ContainSubstring("scope: invalid scope"))

	Expect(CheckResponse(&APIResponse{StatusCode: 204})).To(BeNil())

	err = CheckResponse(&APIResponse{StatusCode: 429})
	Expect(err.Error()).To(ContainSubstring("rate limit"))
}

func TestExecuteAPIBearerAuth(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer tok_x"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resource":{"uri":"u"}}`))
	}))
	defer server.Close()
	restore := SetBaseURLForTest(server.URL)
	defer restore()

	resp, err := ExecuteAPI("tok_x", http.MethodGet, "/users/me", nil)
	Expect(err).To(BeNil())
	Expect(resp.StatusCode).To(Equal(200))
}

func TestListResourcesPagination(t *testing.T) {
	RegisterTestingT(t)
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page++
		switch r.URL.Query().Get("page_token") {
		case "":
			_, _ = w.Write([]byte(`{"collection":[{"n":1},{"n":2}],"pagination":{"next_page_token":"t2"}}`))
		case "t2":
			_, _ = w.Write([]byte(`{"collection":[{"n":3}],"pagination":{"next_page_token":null}}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	restore := SetBaseURLForTest(server.URL)
	defer restore()

	// Single page: returns the cursor for manual resumption.
	items, next, _, pages, err := ListResources("tok", "/event_types", url.Values{}, false)
	Expect(err).To(BeNil())
	Expect(items).To(HaveLen(2))
	Expect(next).To(Equal("t2"))
	Expect(pages).To(Equal(1))

	// Return-all: follows the cursor until drained.
	items, next, _, pages, err = ListResources("tok", "/event_types", url.Values{}, true)
	Expect(err).To(BeNil())
	Expect(items).To(HaveLen(3))
	Expect(next).To(Equal(""))
	Expect(pages).To(Equal(2))
}

func TestListResourcesEmptyCollection(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"collection":[],"pagination":{}}`))
	}))
	defer server.Close()
	restore := SetBaseURLForTest(server.URL)
	defer restore()

	items, next, _, _, err := ListResources("tok", "/event_types", nil, true)
	Expect(err).To(BeNil())
	Expect(items).NotTo(BeNil())
	Expect(items).To(HaveLen(0))
	Expect(next).To(Equal(""))
}

func TestScopeFilter(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/users/me"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"resource":{"uri":"https://api.calendly.com/users/U1","current_organization":"https://api.calendly.com/organizations/O1"}}`)
	}))
	defer server.Close()
	restore := SetBaseURLForTest(server.URL)
	defer restore()

	// Default scope → user filter.
	q := url.Values{}
	Expect(ScopeFilter("tok", []*core.Connection{}, q)).To(BeNil())
	Expect(q.Get("user")).To(Equal("https://api.calendly.com/users/U1"))
	Expect(q.Get("organization")).To(Equal(""))

	// Organization scope → organization filter.
	q = url.Values{}
	inputs := []*core.Connection{{Name: "scope", Type: core.ConnectionTypeString, Value: "organization"}}
	Expect(ScopeFilter("tok", inputs, q)).To(BeNil())
	Expect(q.Get("organization")).To(Equal("https://api.calendly.com/organizations/O1"))
	Expect(q.Get("user")).To(Equal(""))
}

func TestClampLimit(t *testing.T) {
	RegisterTestingT(t)
	Expect(ClampLimit(0, false)).To(Equal(DefaultPageLimit))
	Expect(ClampLimit(0, true)).To(Equal(DefaultPageLimit))
	Expect(ClampLimit(20, true)).To(Equal(20))
	Expect(ClampLimit(500, true)).To(Equal(MaxPageLimit))
}

func TestResourceResult(t *testing.T) {
	RegisterTestingT(t)
	out := ResourceResult(map[string]interface{}{
		"resource": map[string]interface{}{"uri": "https://api.calendly.com/scheduled_events/E1", "name": "Intro Call"},
	}, "summary")
	Expect(out["id"]).To(Equal("https://api.calendly.com/scheduled_events/E1"))
	Expect(out["success"]).To(Equal(true))
	res, _ := out["result"].(map[string]interface{})
	Expect(res["name"]).To(Equal("Intro Call"))
}

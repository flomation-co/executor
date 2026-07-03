package scheduling_calendly_event_get_all

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
	. "github.com/onsi/gomega"
)

func TestExecuteMissingAuth(t *testing.T) {
	RegisterTestingT(t)
	res, err := Execute(&core.Flow{}, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("authentication required"))
	Expect(res).To(BeNil())
}

func TestExecuteListsEvents(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/me":
			_, _ = w.Write([]byte(`{"resource":{"uri":"https://api.calendly.com/users/U1","current_organization":"https://api.calendly.com/organizations/O1"}}`))
		case "/scheduled_events":
			Expect(r.URL.Query().Get("user")).To(Equal("https://api.calendly.com/users/U1"))
			Expect(r.URL.Query().Get("status")).To(Equal("active"))
			Expect(r.URL.Query().Get("count")).To(Equal("50"))
			_, _ = w.Write([]byte(`{"collection":[{"uri":"https://api.calendly.com/scheduled_events/E1","name":"Intro Call"}],"pagination":{"next_page_token":null}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	restore := calendly.SetBaseURLForTest(server.URL)
	defer restore()

	res, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok_x"},
		{Name: "status", Type: core.ConnectionTypeString, Value: "active"},
	})
	Expect(err).To(BeNil())
	Expect(res["success"]).To(Equal(true))
	Expect(res["count"]).To(Equal(1))
	items, _ := res["results"].([]interface{})
	Expect(items).To(HaveLen(1))
}

func TestExecuteOrganizationScope(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/me":
			_, _ = w.Write([]byte(`{"resource":{"uri":"https://api.calendly.com/users/U1","current_organization":"https://api.calendly.com/organizations/O1"}}`))
		case "/scheduled_events":
			Expect(r.URL.Query().Get("organization")).To(Equal("https://api.calendly.com/organizations/O1"))
			Expect(r.URL.Query().Get("user")).To(Equal(""))
			_, _ = w.Write([]byte(`{"collection":[],"pagination":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	restore := calendly.SetBaseURLForTest(server.URL)
	defer restore()

	res, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok_x"},
		{Name: "scope", Type: core.ConnectionTypeString, Value: "organization"},
	})
	Expect(err).To(BeNil())
	Expect(res["success"]).To(Equal(true))
	Expect(res["count"]).To(Equal(0))
}

func TestExecuteAPIError(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"title":"Unauthenticated","message":"The access token is invalid"}`))
	}))
	defer server.Close()
	restore := calendly.SetBaseURLForTest(server.URL)
	defer restore()

	res, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok_bad"},
	})
	Expect(err).To(BeNil()) // soft error
	Expect(res["success"]).To(Equal(false))
	Expect(res["error"]).To(ContainSubstring("access token is invalid"))
}

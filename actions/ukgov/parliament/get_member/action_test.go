package ukgov_parliament_get_member

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestGetMember(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":{"id":4514,"nameDisplayAs":"Keir Starmer","latestParty":{"name":"Labour"},"latestHouseMembership":{"membershipFrom":"Holborn and St Pancras","house":1}}}`))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "member_id", Type: core.ConnectionTypeString, Value: "4514"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(gotPath).To(Equal("/api/Members/4514"))
	Expect(out["name"]).To(Equal("Keir Starmer"))
	Expect(out["party"]).To(Equal("Labour"))
	Expect(out["house"]).To(Equal("Commons"))
	Expect(out["tool_result"]).To(ContainSubstring("Labour MP for Holborn and St Pancras"))
}

func TestGetMemberNotFound(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "member_id", Type: core.ConnectionTypeString, Value: "0"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("No UK Parliament member found"))
}

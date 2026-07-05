package ukgov_parliament_search_members

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

const sample = `{"totalResults":1,"items":[
  {"value":{"id":4514,"nameDisplayAs":"Keir Starmer","gender":"M",
    "latestParty":{"name":"Labour","abbreviation":"Lab"},
    "latestHouseMembership":{"membershipFrom":"Holborn and St Pancras","house":1}}}
]}`

func TestSearchMembers(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "name", Type: core.ConnectionTypeString, Value: "Starmer"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))
	Expect(gotPath).To(Equal("/api/Members/Search"))
	Expect(gotQuery).To(ContainSubstring("Name=Starmer"))
	Expect(out["tool_result"]).To(ContainSubstring("Keir Starmer (Labour — Holborn and St Pancras, Commons)"))
}

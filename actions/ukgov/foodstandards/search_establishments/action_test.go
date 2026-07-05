package ukgov_foodstandards_search_establishments

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

const sampleResponse = `{
  "establishments": [
    {"FHRSID": 1, "BusinessName": "The Ivy", "BusinessType": "Restaurant/Cafe/Canteen", "PostCode": "WC2H 9NQ", "RatingValue": "5", "LocalAuthorityName": "Westminster"},
    {"FHRSID": 2, "BusinessName": "Corner Cafe", "BusinessType": "Restaurant/Cafe/Canteen", "PostCode": "WC2H 8AA", "RatingValue": "4", "LocalAuthorityName": "Westminster"}
  ],
  "meta": {"totalCount": 2}
}`

func TestSearchEstablishments(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotQuery, gotVersion, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotVersion = r.Header.Get("x-api-version")
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleResponse))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "name", Type: core.ConnectionTypeString, Value: "cafe"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	Expect(out["total"]).To(Equal(2))
	Expect(gotPath).To(Equal("/Establishments"))
	Expect(gotQuery).To(ContainSubstring("name=cafe"))
	Expect(gotQuery).To(ContainSubstring("pageSize=10"))
	Expect(gotVersion).To(Equal("2"))
	Expect(gotAccept).To(Equal("application/json"))
	Expect(out["tool_result"]).To(ContainSubstring("The Ivy"))
	Expect(out["tool_result"]).To(ContainSubstring("hygiene rating 5"))
}

func TestSearchEstablishmentsCapsMaxResults(t *testing.T) {
	RegisterTestingT(t)
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"establishments":[],"meta":{"totalCount":0}}`))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	_, err := Execute(nil, nil, []*core.Connection{
		{Name: "name", Type: core.ConnectionTypeString, Value: "x"},
		{Name: "max_results", Type: core.ConnectionTypeInteger, Value: int64(500)},
	})
	Expect(err).To(BeNil())
	Expect(gotQuery).To(ContainSubstring("pageSize=50"))
}

func TestSearchEstablishmentsRequiresQuery(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("business name or address"))
}

func TestSearchEstablishmentsUpstreamError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "name", Type: core.ConnectionTypeString, Value: "x"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("status 500"))
}

func TestSummariseEmpty(t *testing.T) {
	RegisterTestingT(t)
	Expect(summarise(nil, 0, "cafe")).To(ContainSubstring("No food establishments"))
}

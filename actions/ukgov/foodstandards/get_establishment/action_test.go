package ukgov_foodstandards_get_establishment

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/foodstandards"
	. "github.com/onsi/gomega"
)

func mockFHRS(status int, body string, capture func(*http.Request)) func() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capture != nil {
			capture(r)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	old := foodstandards.BaseURL
	foodstandards.BaseURL = srv.URL
	return func() {
		foodstandards.BaseURL = old
		srv.Close()
	}
}

func TestGetEstablishment(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	restore := mockFHRS(http.StatusOK, `{"FHRSID":512112,"BusinessName":"The Ivy","RatingValue":"5","AddressLine1":"1-5 West St","PostCode":"WC2H 9NQ"}`, func(r *http.Request) {
		gotPath = r.URL.Path
	})
	defer restore()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "fhrs_id", Type: core.ConnectionTypeString, Value: "512112"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(gotPath).To(Equal("/Establishments/512112"))
	Expect(out["business_name"]).To(Equal("The Ivy"))
	Expect(out["rating_value"]).To(Equal("5"))
	Expect(out["address"]).To(Equal("1-5 West St, WC2H 9NQ"))
	Expect(out["tool_result"]).To(ContainSubstring("food hygiene rating 5"))
}

func TestGetEstablishmentNotFound(t *testing.T) {
	RegisterTestingT(t)
	restore := mockFHRS(http.StatusNotFound, "", nil)
	defer restore()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "fhrs_id", Type: core.ConnectionTypeString, Value: "999999999"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("No food establishment found"))
}

func TestGetEstablishmentRequiresID(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("FHRS ID is required"))
}

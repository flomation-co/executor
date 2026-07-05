package ukgov_landregistry_price_paid

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

const sample = `{"result":{"items":[
  {"pricePaid":217750,"transactionDate":"2024-05-01","newBuild":false,
   "propertyType":{"_about":"http://landregistry.data.gov.uk/def/common/semi-detached","label":[{"_value":"Semi-detached"}]},
   "estateType":{"_about":"http://landregistry.data.gov.uk/def/common/freehold","label":[{"_value":"Freehold"}]},
   "propertyAddress":{"paon":"104","street":"PATTINSON DRIVE","town":"PLYMOUTH","postcode":"PL6 8RU"}},
  {"pricePaid":180000,"transactionDate":"2022-03-15","newBuild":false,
   "propertyType":{"label":[{"_value":"Terraced"}]},"estateType":{"label":[{"_value":"Freehold"}]},
   "propertyAddress":{"paon":"106","street":"PATTINSON DRIVE","town":"PLYMOUTH","postcode":"PL6 8RU"}}
]}}`

func TestPricePaid(t *testing.T) {
	RegisterTestingT(t)
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "postcode", Type: core.ConnectionTypeString, Value: "pl68ru"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	// Postcode normalised to uppercase with a space, then URL-encoded.
	Expect(gotQuery).To(ContainSubstring("PL6+8RU"))
	Expect(gotQuery).To(ContainSubstring("_sort=-transactionDate"))
	Expect(out["tool_result"]).To(ContainSubstring("£217,750"))
	Expect(out["tool_result"]).To(ContainSubstring("Semi-detached, Freehold"))
	Expect(out["tool_result"]).To(ContainSubstring("PATTINSON DRIVE"))
}

func TestPricePaidNone(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"items":[]}}`))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "postcode", Type: core.ConnectionTypeString, Value: "ZZ1 1ZZ"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(0))
	Expect(out["tool_result"]).To(ContainSubstring("No sold-property records"))
}

func TestNormalisePostcode(t *testing.T) {
	RegisterTestingT(t)
	Expect(normalisePostcode("pl68ru")).To(Equal("PL6 8RU"))
	Expect(normalisePostcode("SW1A 1AA")).To(Equal("SW1A 1AA"))
	Expect(normalisePostcode("ec1a1bb")).To(Equal("EC1A 1BB"))
}

func TestFormatPrice(t *testing.T) {
	RegisterTestingT(t)
	Expect(formatPrice(999)).To(Equal("£999"))
	Expect(formatPrice(217750)).To(Equal("£217,750"))
	Expect(formatPrice(1000000)).To(Equal("£1,000,000"))
}

package ukgov_dvla_vehicle_enquiry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestVehicleEnquiry(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotKey, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-api-key")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"registrationNumber":"AB19ABC","taxStatus":"Taxed","taxDueDate":"2025-03-01","motStatus":"Valid","motExpiryDate":"2025-06-15","make":"FORD","colour":"BLUE","yearOfManufacture":2019,"fuelType":"PETROL","co2Emissions":120}`))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "testkey"},
		{Name: "registration_number", Type: core.ConnectionTypeString, Value: "ab19 abc"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(gotPath).To(Equal("/vehicle-enquiry/v1/vehicles"))
	Expect(gotKey).To(Equal("testkey"))
	// VRN should be normalised: uppercased, spaces stripped.
	var sent map[string]string
	Expect(json.Unmarshal([]byte(gotBody), &sent)).To(Succeed())
	Expect(sent["registrationNumber"]).To(Equal("AB19ABC"))
	Expect(out["make"]).To(Equal("FORD"))
	Expect(out["tax_status"]).To(Equal("Taxed"))
	Expect(out["year_of_manufacture"]).To(Equal(2019))
	Expect(out["tool_result"]).To(ContainSubstring("BLUE FORD"))
	Expect(out["tool_result"]).To(ContainSubstring("Tax: Taxed (due 2025-03-01)"))
	Expect(out["tool_result"]).To(ContainSubstring("MOT: Valid (expires 2025-06-15)"))
	Expect(out["tool_result"]).To(ContainSubstring("120 g/km CO2"))
}

func TestVehicleEnquiryNotFound(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"status":"404","title":"Vehicle Not Found"}]}`))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "testkey"},
		{Name: "registration_number", Type: core.ConnectionTypeString, Value: "ZZ99ZZZ"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("No vehicle found"))
}

func TestVehicleEnquiryBadKey(t *testing.T) {
	RegisterTestingT(t)
	// The 403 path uses the API-Gateway {"message":...} form, not errors[].
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "bad"},
		{Name: "registration_number", Type: core.ConnectionTypeString, Value: "AB19ABC"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("authentication failed"))
}

func TestVehicleEnquiryRequiresReg(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "testkey"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("registration number is required"))
}

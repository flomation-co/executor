package ukgov_dvsa_mot_history

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/dvsa"
	. "github.com/onsi/gomega"
)

// motServer routes the OAuth token exchange and the vehicle lookup to one mock.
func motServer(t *testing.T, vehicleStatus int, vehicleBody string, capture func(tokenHit bool, r *http.Request)) func() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token") {
			if capture != nil {
				capture(true, r)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"tok-123","token_type":"Bearer","expires_in":1199}`))
			return
		}
		if capture != nil {
			capture(false, r)
		}
		w.WriteHeader(vehicleStatus)
		_, _ = w.Write([]byte(vehicleBody))
	}))
	oldBase, oldLogin := dvsa.BaseURL, dvsa.LoginBaseURL
	dvsa.BaseURL = srv.URL
	dvsa.LoginBaseURL = srv.URL
	return func() {
		dvsa.BaseURL, dvsa.LoginBaseURL = oldBase, oldLogin
		srv.Close()
	}
}

func creds(reg string) []*core.Connection {
	return []*core.Connection{
		{Name: "client_id", Type: core.ConnectionTypeSecret, Value: "cid"},
		{Name: "client_secret", Type: core.ConnectionTypeSecret, Value: "csec"},
		{Name: "tenant_id", Type: core.ConnectionTypeString, Value: "mytenant"},
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "apikey"},
		{Name: "registration_number", Type: core.ConnectionTypeString, Value: reg},
	}
}

func TestMotHistory(t *testing.T) {
	RegisterTestingT(t)
	var vehiclePath, gotAuth, gotAPIKey string
	restore := motServer(t, http.StatusOK, `{"registration":"AB19ABC","make":"FORD","model":"FOCUS","fuelType":"PETROL","primaryColour":"Blue","motTests":[
	  {"completedDate":"2024-06-15T10:00:00","testResult":"PASSED","expiryDate":"2025-06-15","odometerValue":"42000","odometerUnit":"mi","motTestNumber":"1234"}
	]}`, func(tokenHit bool, r *http.Request) {
		if !tokenHit {
			vehiclePath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			gotAPIKey = r.Header.Get("X-API-Key")
		}
	})
	defer restore()

	out, err := Execute(nil, nil, creds("ab19 abc"))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	// VRN normalised, correct endpoint + headers (token from the exchange).
	Expect(vehiclePath).To(Equal("/v1/trade/vehicles/registration/AB19ABC"))
	Expect(gotAuth).To(Equal("Bearer tok-123"))
	Expect(gotAPIKey).To(Equal("apikey"))
	Expect(out["make"]).To(Equal("FORD"))
	Expect(out["latest_result"]).To(Equal("PASSED"))
	Expect(out["tool_result"]).To(ContainSubstring("AB19ABC: FORD FOCUS (PETROL, Blue)"))
	Expect(out["tool_result"]).To(ContainSubstring("Latest MOT: PASSED on 2024-06-15 (expires 2025-06-15), 42000 mi"))
	Expect(out["tool_result"]).To(ContainSubstring("1 test(s) on record"))
}

func TestMotHistoryNotFound(t *testing.T) {
	RegisterTestingT(t)
	restore := motServer(t, http.StatusNotFound, `{"errors":[{"code":"MOTH-NF-01"}]}`, nil)
	defer restore()

	out, err := Execute(nil, nil, creds("ZZ99ZZZ"))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("No MOT history found"))
}

func TestMotHistoryRequiresReg(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "client_id", Type: core.ConnectionTypeSecret, Value: "cid"},
		{Name: "client_secret", Type: core.ConnectionTypeSecret, Value: "csec"},
		{Name: "tenant_id", Type: core.ConnectionTypeString, Value: "t"},
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "k"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("registration number is required"))
}

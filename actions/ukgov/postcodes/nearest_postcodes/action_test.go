package ukgov_postcodes_nearest_postcodes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/postcodes"
	. "github.com/onsi/gomega"
)

func TestNearestPostcodes(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":200,"result":[{"postcode":"SW1A 1AA"},{"postcode":"SW1A 2AA"},{"postcode":"SW1A 2AB"}]}`))
	}))
	defer srv.Close()

	old := postcodes.BaseURL
	postcodes.BaseURL = srv.URL
	defer func() { postcodes.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "postcode", Type: core.ConnectionTypeString, Value: "SW1A 1AA"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(3))
	Expect(gotPath).To(Equal("/postcodes/SW1A%201AA/nearest"))
	Expect(out["tool_result"]).To(ContainSubstring("SW1A 2AA"))
}

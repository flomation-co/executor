package ukgov_postcodes_validate_postcode

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/postcodes"
	. "github.com/onsi/gomega"
)

func mockValidate(result string) func() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":200,"result":` + result + `}`))
	}))
	old := postcodes.BaseURL
	postcodes.BaseURL = srv.URL
	return func() {
		postcodes.BaseURL = old
		srv.Close()
	}
}

func TestValidatePostcodeValid(t *testing.T) {
	RegisterTestingT(t)
	restore := mockValidate("true")
	defer restore()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "postcode", Type: core.ConnectionTypeString, Value: "SW1A 1AA"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["valid"]).To(Equal(true))
	Expect(out["tool_result"]).To(ContainSubstring("is a valid"))
}

func TestValidatePostcodeInvalid(t *testing.T) {
	RegisterTestingT(t)
	restore := mockValidate("false")
	defer restore()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "postcode", Type: core.ConnectionTypeString, Value: "NOPE"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["valid"]).To(Equal(false))
	Expect(out["tool_result"]).To(ContainSubstring("is not a valid"))
}

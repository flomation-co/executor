package ukgov_foodstandards_list_business_types

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/foodstandards"
	. "github.com/onsi/gomega"
)

func TestListBusinessTypes(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/BusinessTypes"))
		Expect(r.Header.Get("x-api-version")).To(Equal("2"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"businessTypes":[{"BusinessTypeId":1,"BusinessTypeName":"Restaurant/Cafe/Canteen"},{"BusinessTypeId":7838,"BusinessTypeName":"Retailers - other"}]}`))
	}))
	defer srv.Close()

	old := foodstandards.BaseURL
	foodstandards.BaseURL = srv.URL
	defer func() { foodstandards.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	Expect(out["tool_result"]).To(ContainSubstring("Restaurant/Cafe/Canteen"))
	Expect(out["tool_result"]).To(ContainSubstring("Retailers - other"))
}

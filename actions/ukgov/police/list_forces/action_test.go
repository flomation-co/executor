package ukgov_police_list_forces

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/police"
	. "github.com/onsi/gomega"
)

func TestListForces(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/forces"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"leicestershire","name":"Leicestershire Police"},{"id":"metropolitan","name":"Metropolitan Police Service"}]`))
	}))
	defer srv.Close()

	old := police.BaseURL
	police.BaseURL = srv.URL
	defer func() { police.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	Expect(out["tool_result"]).To(ContainSubstring("Leicestershire Police"))
}

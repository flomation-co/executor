package estimate_create

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
	. "github.com/onsi/gomega"
)

func TestEstimateCreate(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Estimate":{"Id":"145","DocNumber":"EST-1001"}}`))
	}))
	defer srv.Close()

	old := quickbooks_common.ProductionBaseURL
	quickbooks_common.ProductionBaseURL = srv.URL
	defer func() { quickbooks_common.ProductionBaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok-abc"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "9130347"},
		{Name: "customer_id", Type: core.ConnectionTypeString, Value: "42"},
		{Name: "line_items", Type: core.ConnectionTypeText, Value: `[{"Amount":100.0,"DetailType":"SalesItemLineDetail","SalesItemLineDetail":{"ItemRef":{"value":"1"}}}]`},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("145"))
	Expect(gotAuth).To(Equal("Bearer tok-abc"))
	Expect(gotPath).To(Equal("/v3/company/9130347/estimate"))

	var sent map[string]interface{}
	Expect(json.Unmarshal([]byte(gotBody), &sent)).To(Succeed())
	custRef, _ := sent["CustomerRef"].(map[string]interface{})
	Expect(custRef["value"]).To(Equal("42"))
	lines, _ := sent["Line"].([]interface{})
	Expect(lines).To(HaveLen(1))
}

func TestEstimateCreateSandboxRouting(t *testing.T) {
	RegisterTestingT(t)
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Estimate":{"Id":"1"}}`))
	}))
	defer srv.Close()

	old := quickbooks_common.SandboxBaseURL
	quickbooks_common.SandboxBaseURL = srv.URL
	defer func() { quickbooks_common.SandboxBaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "1"},
		{Name: "sandbox", Type: core.ConnectionTypeBoolean, Value: true},
		{Name: "customer_id", Type: core.ConnectionTypeString, Value: "42"},
		{Name: "line_items", Type: core.ConnectionTypeText, Value: `[{"Amount":100.0}]`},
	})
	Expect(err).To(BeNil())
	Expect(hit).To(BeTrue())
	Expect(out["success"]).To(Equal(true))
}

func TestEstimateCreateMissingLineItems(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "1"},
		{Name: "customer_id", Type: core.ConnectionTypeString, Value: "42"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(strings.ToLower(out["error"].(string))).To(ContainSubstring("line_items is required"))
}

func TestEstimateCreateFaultError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"Fault":{"Error":[{"Message":"Invalid Reference Id","Detail":"Object Not Found","code":"610"}],"type":"ValidationFault"}}`))
	}))
	defer srv.Close()

	old := quickbooks_common.ProductionBaseURL
	quickbooks_common.ProductionBaseURL = srv.URL
	defer func() { quickbooks_common.ProductionBaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "1"},
		{Name: "customer_id", Type: core.ConnectionTypeString, Value: "999"},
		{Name: "line_items", Type: core.ConnectionTypeText, Value: `[{"Amount":100.0}]`},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Invalid Reference Id"))
}

func TestEstimateCreateUnresolvedCredential(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "${credentials.MyQBO}"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "${credentials.MyQBO.realm_id}"},
		{Name: "customer_id", Type: core.ConnectionTypeString, Value: "42"},
		{Name: "line_items", Type: core.ConnectionTypeText, Value: `[{"Amount":100.0}]`},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(strings.ToLower(out["error"].(string))).To(ContainSubstring("connect and authorise a quickbooks account"))
}

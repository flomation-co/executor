package customer_create

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

func TestCustomerCreate(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Customer":{"Id":"42","DisplayName":"Ada Lovelace Ltd"}}`))
	}))
	defer srv.Close()

	old := quickbooks_common.ProductionBaseURL
	quickbooks_common.ProductionBaseURL = srv.URL
	defer func() { quickbooks_common.ProductionBaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok-abc"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "9130347"},
		{Name: "display_name", Type: core.ConnectionTypeString, Value: "Ada Lovelace Ltd"},
		{Name: "email", Type: core.ConnectionTypeString, Value: "ada@example.com"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("42"))
	Expect(gotAuth).To(Equal("Bearer tok-abc"))
	Expect(gotPath).To(Equal("/v3/company/9130347/customer"))

	var sent map[string]interface{}
	Expect(json.Unmarshal([]byte(gotBody), &sent)).To(Succeed())
	Expect(sent["DisplayName"]).To(Equal("Ada Lovelace Ltd"))
	email, _ := sent["PrimaryEmailAddr"].(map[string]interface{})
	Expect(email["Address"]).To(Equal("ada@example.com"))
}

func TestCustomerCreateSandboxRouting(t *testing.T) {
	RegisterTestingT(t)
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Customer":{"Id":"1","DisplayName":"Test"}}`))
	}))
	defer srv.Close()

	old := quickbooks_common.SandboxBaseURL
	quickbooks_common.SandboxBaseURL = srv.URL
	defer func() { quickbooks_common.SandboxBaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "1"},
		{Name: "sandbox", Type: core.ConnectionTypeBoolean, Value: true},
		{Name: "display_name", Type: core.ConnectionTypeString, Value: "Test"},
	})
	Expect(err).To(BeNil())
	Expect(hit).To(BeTrue())
	Expect(out["success"]).To(Equal(true))
}

func TestCustomerCreateFaultError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"Fault":{"Error":[{"Message":"Duplicate Name Exists Error","Detail":"The name supplied already exists.","code":"6240"}],"type":"ValidationFault"}}`))
	}))
	defer srv.Close()

	old := quickbooks_common.ProductionBaseURL
	quickbooks_common.ProductionBaseURL = srv.URL
	defer func() { quickbooks_common.ProductionBaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "1"},
		{Name: "display_name", Type: core.ConnectionTypeString, Value: "Dupe"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(SatisfyAll(
		ContainSubstring("Duplicate Name Exists Error"),
		ContainSubstring("already exists"),
	))
}

func TestCustomerCreateUnresolvedCredential(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "${credentials.MyQBO}"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "${credentials.MyQBO.realm_id}"},
		{Name: "display_name", Type: core.ConnectionTypeString, Value: "Ada"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(strings.ToLower(out["error"].(string))).To(ContainSubstring("connect and authorise a quickbooks account"))
}

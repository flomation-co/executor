package contact_create

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
	. "github.com/onsi/gomega"
)

func TestContactCreate(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotAuth, gotTenant, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotTenant = r.Header.Get("Xero-Tenant-Id")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Contacts":[{"ContactID":"c-123","Name":"Ada Lovelace Ltd","EmailAddress":"ada@example.com"}]}`))
	}))
	defer srv.Close()

	old := xero_common.BaseURL
	xero_common.BaseURL = srv.URL
	defer func() { xero_common.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok-abc"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "ten-xyz"},
		{Name: "name", Type: core.ConnectionTypeString, Value: "Ada Lovelace Ltd"},
		{Name: "email", Type: core.ConnectionTypeString, Value: "ada@example.com"},
		{Name: "fields", Type: core.ConnectionTypeText, Value: `{"AccountNumber":"AL-01"}`},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("c-123"))
	Expect(gotPath).To(Equal("/Contacts"))
	Expect(gotAuth).To(Equal("Bearer tok-abc"))
	Expect(gotTenant).To(Equal("ten-xyz"))

	var sent map[string]interface{}
	Expect(json.Unmarshal([]byte(gotBody), &sent)).To(Succeed())
	Expect(sent["Name"]).To(Equal("Ada Lovelace Ltd"))
	Expect(sent["EmailAddress"]).To(Equal("ada@example.com"))
	// advanced JSON override merged in
	Expect(sent["AccountNumber"]).To(Equal("AL-01"))
}

func TestContactCreateValidationError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"Type":"ValidationException","Message":"A validation exception occurred","Elements":[{"ValidationErrors":[{"Message":"Contact Name must be specified"}]}]}`))
	}))
	defer srv.Close()

	old := xero_common.BaseURL
	xero_common.BaseURL = srv.URL
	defer func() { xero_common.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "ten"},
		{Name: "name", Type: core.ConnectionTypeString, Value: "x"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Contact Name must be specified"))
}

func TestContactCreateUnresolvedCredential(t *testing.T) {
	RegisterTestingT(t)
	// A ${credentials...} reference that failed to resolve must surface a
	// friendly error, not fire a request with a literal placeholder token.
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "${credentials.MyXero}"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "${credentials.MyXero.tenant_id}"},
		{Name: "name", Type: core.ConnectionTypeString, Value: "Ada"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("connect and authorise a Xero account"))
}

package invoice_create

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

func TestInvoiceCreate(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotAuth, gotTenant, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotTenant = r.Header.Get("Xero-Tenant-Id")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Invoices":[{"InvoiceID":"inv-123","Type":"ACCREC","Status":"DRAFT"}]}`))
	}))
	defer srv.Close()

	old := xero_common.BaseURL
	xero_common.BaseURL = srv.URL
	defer func() { xero_common.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok-abc"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "ten-xyz"},
		{Name: "contact_id", Type: core.ConnectionTypeString, Value: "c-123"},
		{Name: "line_items", Type: core.ConnectionTypeText, Value: `[{"Description":"Consulting","Quantity":1,"UnitAmount":100.00,"AccountCode":"200"}]`},
		{Name: "reference", Type: core.ConnectionTypeString, Value: "INV-001"},
		{Name: "fields", Type: core.ConnectionTypeText, Value: `{"CurrencyCode":"GBP"}`},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("inv-123"))
	Expect(gotPath).To(Equal("/Invoices"))
	Expect(gotAuth).To(Equal("Bearer tok-abc"))
	Expect(gotTenant).To(Equal("ten-xyz"))

	var sent map[string]interface{}
	Expect(json.Unmarshal([]byte(gotBody), &sent)).To(Succeed())
	// default type applied
	Expect(sent["Type"]).To(Equal("ACCREC"))
	// contact_id mapped to Contact:{ContactID}
	contact, ok := sent["Contact"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(contact["ContactID"]).To(Equal("c-123"))
	// line_items mapped to LineItems
	lines, ok := sent["LineItems"].([]interface{})
	Expect(ok).To(BeTrue())
	Expect(lines).To(HaveLen(1))
	Expect(sent["Reference"]).To(Equal("INV-001"))
	// advanced JSON override merged in
	Expect(sent["CurrencyCode"]).To(Equal("GBP"))
}

func TestInvoiceCreateMissingLineItems(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "ten"},
		{Name: "contact_id", Type: core.ConnectionTypeString, Value: "c-1"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("line_items is required"))
}

func TestInvoiceCreateUnresolvedCredential(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "${credentials.MyXero}"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "${credentials.MyXero.tenant_id}"},
		{Name: "contact_id", Type: core.ConnectionTypeString, Value: "c-1"},
		{Name: "line_items", Type: core.ConnectionTypeText, Value: `[{"Description":"x"}]`},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("connect and authorise a Xero account"))
}

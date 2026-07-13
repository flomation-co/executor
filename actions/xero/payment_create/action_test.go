package payment_create

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

func TestPaymentCreate(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotAuth, gotTenant, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotTenant = r.Header.Get("Xero-Tenant-Id")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Payments":[{"PaymentID":"p-123","Amount":100.0}]}`))
	}))
	defer srv.Close()

	old := xero_common.BaseURL
	xero_common.BaseURL = srv.URL
	defer func() { xero_common.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok-abc"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "ten-xyz"},
		{Name: "invoice_id", Type: core.ConnectionTypeString, Value: "inv-1"},
		{Name: "account_id", Type: core.ConnectionTypeString, Value: "acc-1"},
		{Name: "amount", Type: core.ConnectionTypeMoney, Value: "100.00"},
		{Name: "date", Type: core.ConnectionTypeString, Value: "2026-07-05"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("p-123"))
	Expect(gotPath).To(Equal("/Payments"))
	Expect(gotAuth).To(Equal("Bearer tok-abc"))
	Expect(gotTenant).To(Equal("ten-xyz"))

	var sent map[string]interface{}
	Expect(json.Unmarshal([]byte(gotBody), &sent)).To(Succeed())
	Expect(sent["Amount"]).To(Equal(100.0))
	inv, _ := sent["Invoice"].(map[string]interface{})
	Expect(inv["InvoiceID"]).To(Equal("inv-1"))
	acc, _ := sent["Account"].(map[string]interface{})
	Expect(acc["AccountID"]).To(Equal("acc-1"))
}

func TestPaymentCreateValidationError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"Type":"ValidationException","Message":"A validation exception occurred","Elements":[{"ValidationErrors":[{"Message":"Payment amount exceeds the amount outstanding"}]}]}`))
	}))
	defer srv.Close()

	old := xero_common.BaseURL
	xero_common.BaseURL = srv.URL
	defer func() { xero_common.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "ten"},
		{Name: "invoice_id", Type: core.ConnectionTypeString, Value: "inv-1"},
		{Name: "account_id", Type: core.ConnectionTypeString, Value: "acc-1"},
		{Name: "amount", Type: core.ConnectionTypeMoney, Value: "999.00"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Payment amount exceeds"))
}

func TestPaymentCreateUnresolvedCredential(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "${credentials.MyXero}"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "${credentials.MyXero.tenant_id}"},
		{Name: "invoice_id", Type: core.ConnectionTypeString, Value: "inv-1"},
		{Name: "account_id", Type: core.ConnectionTypeString, Value: "acc-1"},
		{Name: "amount", Type: core.ConnectionTypeMoney, Value: "100.00"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("connect and authorise a Xero account"))
}

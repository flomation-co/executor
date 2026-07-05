package invoice_create

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

func TestInvoiceCreate(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Invoice":{"Id":"130","TotalAmt":100.0}}`))
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
		{Name: "txn_date", Type: core.ConnectionTypeString, Value: "2026-07-05"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("130"))
	Expect(gotAuth).To(Equal("Bearer tok-abc"))
	Expect(gotPath).To(Equal("/v3/company/9130347/invoice"))

	var sent map[string]interface{}
	Expect(json.Unmarshal([]byte(gotBody), &sent)).To(Succeed())
	ref, _ := sent["CustomerRef"].(map[string]interface{})
	Expect(ref["value"]).To(Equal("42"))
	lines, _ := sent["Line"].([]interface{})
	Expect(lines).To(HaveLen(1))
	Expect(sent["TxnDate"]).To(Equal("2026-07-05"))
}

func TestInvoiceCreateMissingLines(t *testing.T) {
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

func TestInvoiceCreateUnresolvedCredential(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "${credentials.MyQBO}"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "${credentials.MyQBO.realm_id}"},
		{Name: "customer_id", Type: core.ConnectionTypeString, Value: "42"},
		{Name: "line_items", Type: core.ConnectionTypeText, Value: `[{"Amount":1.0}]`},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(strings.ToLower(out["error"].(string))).To(ContainSubstring("connect and authorise a quickbooks account"))
}

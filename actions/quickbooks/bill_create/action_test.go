package bill_create

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

func TestBillCreate(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Bill":{"Id":"145","TotalAmt":100.00}}`))
	}))
	defer srv.Close()

	old := quickbooks_common.ProductionBaseURL
	quickbooks_common.ProductionBaseURL = srv.URL
	defer func() { quickbooks_common.ProductionBaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok-abc"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "9130347"},
		{Name: "vendor_id", Type: core.ConnectionTypeString, Value: "56"},
		{Name: "line_items", Type: core.ConnectionTypeText, Value: `[{"Amount":100.00,"DetailType":"AccountBasedExpenseLineDetail","AccountBasedExpenseLineDetail":{"AccountRef":{"value":"7"}}}]`},
		{Name: "txn_date", Type: core.ConnectionTypeString, Value: "2026-07-05"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("145"))
	Expect(gotAuth).To(Equal("Bearer tok-abc"))
	Expect(gotPath).To(Equal("/v3/company/9130347/bill"))

	var sent map[string]interface{}
	Expect(json.Unmarshal([]byte(gotBody), &sent)).To(Succeed())
	vendor, _ := sent["VendorRef"].(map[string]interface{})
	Expect(vendor["value"]).To(Equal("56"))
	Expect(sent["TxnDate"]).To(Equal("2026-07-05"))
	lines, _ := sent["Line"].([]interface{})
	Expect(lines).To(HaveLen(1))
}

func TestBillCreateMissingLines(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "1"},
		{Name: "vendor_id", Type: core.ConnectionTypeString, Value: "56"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(strings.ToLower(out["error"].(string))).To(ContainSubstring("line_items is required"))
}

func TestBillCreateFaultError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"Fault":{"Error":[{"Message":"Invalid Reference Id","Detail":"Vendor not found.","code":"610"}],"type":"ValidationFault"}}`))
	}))
	defer srv.Close()

	old := quickbooks_common.ProductionBaseURL
	quickbooks_common.ProductionBaseURL = srv.URL
	defer func() { quickbooks_common.ProductionBaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "1"},
		{Name: "vendor_id", Type: core.ConnectionTypeString, Value: "999"},
		{Name: "line_items", Type: core.ConnectionTypeText, Value: `[{"Amount":10.00}]`},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(SatisfyAll(
		ContainSubstring("Invalid Reference Id"),
		ContainSubstring("Vendor not found"),
	))
}

func TestBillCreateUnresolvedCredential(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "${credentials.MyQBO}"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "${credentials.MyQBO.realm_id}"},
		{Name: "vendor_id", Type: core.ConnectionTypeString, Value: "56"},
		{Name: "line_items", Type: core.ConnectionTypeText, Value: `[{"Amount":10.00}]`},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(strings.ToLower(out["error"].(string))).To(ContainSubstring("connect and authorise a quickbooks account"))
}

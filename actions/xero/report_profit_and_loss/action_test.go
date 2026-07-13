package report_profit_and_loss

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
	. "github.com/onsi/gomega"
)

func TestReportProfitAndLoss(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotAuth, gotTenant, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotTenant = r.Header.Get("Xero-Tenant-Id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Reports":[{"ReportID":"ProfitAndLoss","ReportName":"Profit and Loss"}]}`))
	}))
	defer srv.Close()

	old := xero_common.BaseURL
	xero_common.BaseURL = srv.URL
	defer func() { xero_common.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok-abc"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "ten-xyz"},
		{Name: "from_date", Type: core.ConnectionTypeString, Value: "2026-01-01"},
		{Name: "to_date", Type: core.ConnectionTypeString, Value: "2026-12-31"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(gotPath).To(Equal("/Reports/ProfitAndLoss"))
	Expect(gotQuery).To(ContainSubstring("fromDate=2026-01-01"))
	Expect(gotQuery).To(ContainSubstring("toDate=2026-12-31"))
	Expect(gotAuth).To(Equal("Bearer tok-abc"))
	Expect(gotTenant).To(Equal("ten-xyz"))

	// the whole decoded report body is returned as the result object
	result, ok := out["result"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	reports, ok := result["Reports"].([]interface{})
	Expect(ok).To(BeTrue())
	Expect(reports).To(HaveLen(1))
}

func TestReportProfitAndLossUnresolvedCredential(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "${credentials.MyXero}"},
		{Name: "tenant", Type: core.ConnectionTypeString, Value: "${credentials.MyXero.tenant_id}"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("connect and authorise a Xero account"))
}

package report_profit_and_loss

import (
	"net/url"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Report: Profit and Loss"
	Description  = "Run the Xero Profit and Loss report for an optional date range. Returns the full report."
	Website      = "https://www.flomation.co"
	Icon         = "xero+chart-line"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "from_date", Type: core.ConnectionTypeString, Label: "From Date", Placeholder: "2026-01-01"},
	{Name: "to_date", Type: core.ConnectionTypeString, Label: "To Date", Placeholder: "2026-12-31"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	if v := xero_common.OptionalString("from_date", inputs); v != "" {
		q.Set("fromDate", v)
	}
	if v := xero_common.OptionalString("to_date", inputs); v != "" {
		q.Set("toDate", v)
	}
	path := "/Reports/ProfitAndLoss"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	resp, err := xero_common.DoJSON(flow, "GET", path, token, tenant, nil)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	return xero_common.ObjectResult("", resp, "Fetched Xero Profit and Loss report"), nil
}

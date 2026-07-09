package report

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Report"
	Description  = "Fetch a QuickBooks Online report (e.g. ProfitAndLoss, BalanceSheet) with an optional date range."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+chart-line"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "report_name", Type: core.ConnectionTypeString, Label: "Report Name", Placeholder: "ProfitAndLoss", Required: true},
	{Name: "start_date", Type: core.ConnectionTypeString, Label: "Start Date", Placeholder: "2026-01-01"},
	{Name: "end_date", Type: core.ConnectionTypeString, Label: "End Date", Placeholder: "2026-12-31"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	name, err := quickbooks_common.RequiredString("report_name", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	params := url.Values{}
	if v := quickbooks_common.OptionalString("start_date", inputs); v != "" {
		params.Set("start_date", v)
	}
	if v := quickbooks_common.OptionalString("end_date", inputs); v != "" {
		params.Set("end_date", v)
	}

	resp, err := quickbooks_common.Report(flow, auth, name, params)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	return quickbooks_common.ObjectResult("", resp, fmt.Sprintf("Fetched QuickBooks %s report", name)), nil
}

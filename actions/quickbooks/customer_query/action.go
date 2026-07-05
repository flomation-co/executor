package customer_query

import (
	"fmt"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Customer: Query"
	Description  = "Query QuickBooks Online customers with SQL-like syntax. Returns matching rows."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+magnifying-glass"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Query (SQL)", Placeholder: "select * from Customer where Active = true"},
}

var Outputs = quickbooks_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	sql := quickbooks_common.OptionalString("query", inputs)
	if sql == "" {
		sql = "select * from Customer"
	}

	resp, err := quickbooks_common.Query(flow, auth, sql)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	rows := quickbooks_common.QueryRows(resp, "Customer")
	return quickbooks_common.ListResult(rows, fmt.Sprintf("Found %d QuickBooks customer(s)", len(rows))), nil
}

package purchaseorder_query

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Purchase Order: Query"
	Description  = "Query QuickBooks Online purchase orders with SQL-like syntax. Returns matching purchase order rows."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+magnifying-glass"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Query (SQL)", Placeholder: "select * from PurchaseOrder where VendorRef = '56'"},
	{Name: "where", Type: core.ConnectionTypeString, Label: "Where Clause", Placeholder: "VendorRef = '56'"},
	{Name: "max_results", Type: core.ConnectionTypeString, Label: "Max Results", Placeholder: "100"},
}

var Outputs = quickbooks_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	sql := strings.TrimSpace(quickbooks_common.OptionalString("query", inputs))
	if sql == "" {
		sql = "select * from PurchaseOrder"
		if where := strings.TrimSpace(quickbooks_common.OptionalString("where", inputs)); where != "" {
			sql += " where " + where
		}
		if max := strings.TrimSpace(quickbooks_common.OptionalString("max_results", inputs)); max != "" {
			sql += " maxresults " + max
		}
	}

	resp, err := quickbooks_common.Query(flow, auth, sql)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	rows := quickbooks_common.QueryRows(resp, "PurchaseOrder")
	return quickbooks_common.ListResult(rows, fmt.Sprintf("Found %d QuickBooks purchase order(s)", len(rows))), nil
}

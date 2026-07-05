package journalentry_query

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Journal Entry: Query"
	Description  = "Query QuickBooks Online journal entries. Supply a WHERE clause or a full SQL query."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+magnifying-glass"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "where", Type: core.ConnectionTypeString, Label: "WHERE Clause", Placeholder: "TxnDate > '2026-01-01'"},
	{Name: "max_results", Type: core.ConnectionTypeString, Label: "Max Results", Placeholder: "100"},
	{Name: "query", Type: core.ConnectionTypeText, Label: "Full SQL Query (overrides WHERE)", Placeholder: "select * from JournalEntry"},
}

var Outputs = quickbooks_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	sql := buildQuery("JournalEntry", inputs)

	resp, err := quickbooks_common.Query(flow, auth, sql)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	rows := quickbooks_common.QueryRows(resp, "JournalEntry")
	return quickbooks_common.ListResult(rows, fmt.Sprintf("Found %d journal entries", len(rows))), nil
}

// buildQuery constructs a QBO SQL query for the given entity from the optional
// `query`, `where` and `max_results` inputs.
func buildQuery(entity string, inputs []*core.Connection) string {
	if q := strings.TrimSpace(quickbooks_common.OptionalString("query", inputs)); q != "" {
		return q
	}
	sql := "select * from " + entity
	if where := strings.TrimSpace(quickbooks_common.OptionalString("where", inputs)); where != "" {
		sql += " where " + where
	}
	if mr := strings.TrimSpace(quickbooks_common.OptionalString("max_results", inputs)); mr != "" {
		sql += " maxresults " + mr
	}
	return sql
}

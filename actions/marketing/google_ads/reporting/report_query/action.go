// Package report_query runs a raw GAQL query against a Google Ads account.
package report_query

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Report: Run GAQL Query"
	Description  = "Run any Google Ads Query Language (GAQL) query and return the rows."
	Summary      = "Run a raw GAQL query"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+code"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

// The escape hatch. The curated actions cover the common ground, but the Google
// Ads reporting surface is hundreds of resources wide and any fixed set of
// actions will fall short of something — so rather than pretend otherwise, the
// full query language is available, and Report: List Fields exists next door to
// make it usable without memorising the schema.
var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "query", Type: core.ConnectionTypeCode, Label: "GAQL Query", Required: true, Placeholder: "SELECT campaign.name, metrics.clicks FROM campaign WHERE segments.date DURING LAST_30_DAYS"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Rows"},
	{Name: "table", Type: core.ConnectionTypeObject, Label: "Rows (flattened)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "query", Type: core.ConnectionTypeString, Label: "GAQL Query Used"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, customerID, login, err := gads.GetAuth(inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	query := strings.TrimSpace(gads.OptionalString("query", inputs))
	if query == "" {
		return gads.ErrorResult("a GAQL query is required"), nil
	}
	// GAQL only reads — the search endpoint cannot mutate anything, whatever it
	// is handed — but rejecting a non-SELECT here turns a puzzling server error
	// into a plain one for anyone who arrives expecting SQL.
	if !strings.HasPrefix(strings.ToUpper(query), "SELECT") {
		return gads.ErrorResult("a GAQL query must start with SELECT — GAQL only reads data, and changes are made with the Campaign, Ad Group, Ad and Keyword actions"), nil
	}

	rows, err := gads.NewClient(token, login).Search(flow, customerID, query)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	return gads.RowsResult(rows, fmt.Sprintf("Query returned %d row(s)", len(rows)), map[string]interface{}{"query": query}), nil
}

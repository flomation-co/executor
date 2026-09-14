// Package account_get reads one Google Ads account's settings.
package account_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Account: Get"
	Description  = "Read a Google Ads account's name, currency, timezone and status."
	Summary      = "Read a Google Ads account"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+magnifying-glass"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

const accountQuery = "SELECT customer.id, customer.descriptive_name, customer.currency_code, customer.time_zone, " +
	"customer.manager, customer.test_account, customer.status, customer.auto_tagging_enabled, " +
	"customer.optimization_score, customer.tracking_url_template FROM customer LIMIT 1"

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Account"},
	{Name: "table", Type: core.ConnectionTypeObject, Label: "Account (flattened)"},
	{Name: "currency_code", Type: core.ConnectionTypeString, Label: "Currency"},
	{Name: "time_zone", Type: core.ConnectionTypeString, Label: "Timezone"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, customerID, login, err := gads.GetAuth(inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	rows, err := gads.NewClient(token, login).Search(flow, customerID, accountQuery)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	if len(rows) == 0 {
		return gads.ErrorResult(fmt.Sprintf("account %s was not found, or this connection cannot reach it", customerID)), nil
	}

	// The currency and timezone are surfaced as their own outputs because both
	// change how everything else in a flow must be read: budgets are in the
	// account's currency, and segments.date is in the account's timezone rather
	// than the flow's or UTC.
	flat := gads.FlattenRow(rows[0])
	return gads.RowsResult(rows, "Read Google Ads account "+customerID, map[string]interface{}{
		"currency_code": flat["customer.currency_code"],
		"time_zone":     flat["customer.time_zone"],
	}), nil
}

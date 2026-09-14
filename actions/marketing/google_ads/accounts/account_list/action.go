// Package account_list lists the Google Ads accounts the connected login can
// reach.
package account_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Accounts: List"
	Description  = "List the Google Ads accounts this connection can reach, with name, currency and timezone."
	Summary      = "List reachable Google Ads accounts"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+list"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

// listAccessibleCustomers returns bare resource names and no descriptive names,
// so it is never useful alone — each id is followed up with a one-row query to
// get something a person can recognise. Accounts are few (an agency's clients
// hang off customer_client, not off here), so the extra calls are cheap.
const accountQuery = "SELECT customer.id, customer.descriptive_name, customer.currency_code, customer.time_zone, " +
	"customer.manager, customer.test_account, customer.status, customer.auto_tagging_enabled FROM customer LIMIT 1"

// A guard against an unexpectedly large login. Operation quota is the scarce
// resource on the default Explorer access level (2,880 a day), and burning it
// enumerating accounts would be a poor trade.
const maxAccounts = 100

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Accounts"},
	{Name: "table", Type: core.ConnectionTypeObject, Label: "Accounts (flattened)"},
	{Name: "customer_ids", Type: core.ConnectionTypeObject, Label: "Customer IDs"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := gads.RequiredString("credential", inputs)
	if err != nil {
		return gads.ErrorResult("connect a Google Ads account"), nil
	}
	login := gads.OptionalString("login_customer_id", inputs)

	client := gads.NewClient(token, login)
	ids, err := client.ListAccessibleCustomers(flow)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	if len(ids) == 0 {
		return gads.ErrorResult("this Google Ads login cannot reach any accounts — check the account has access in Google Ads, then reconnect"), nil
	}
	if len(ids) > maxAccounts {
		ids = ids[:maxAccounts]
	}

	var rows []map[string]interface{}
	for _, id := range ids {
		// A single unreadable account must not hide the rest: a cancelled or
		// suspended account in the list is common and is not a failure of the
		// action.
		found, err := client.Search(flow, id, accountQuery)
		if err != nil {
			rows = append(rows, map[string]interface{}{
				"customer": map[string]interface{}{"id": id, "descriptiveName": "(could not be read: " + err.Error() + ")"},
			})
			continue
		}
		rows = append(rows, found...)
	}

	return gads.RowsResult(rows, fmt.Sprintf("Found %d Google Ads account(s)", len(rows)), map[string]interface{}{
		"customer_ids": ids,
	}), nil
}

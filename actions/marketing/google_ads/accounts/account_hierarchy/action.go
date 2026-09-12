// Package account_hierarchy lists the client accounts beneath a Google Ads
// manager account.
package account_hierarchy

import (
	"fmt"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Accounts: Hierarchy"
	Description  = "List the client accounts beneath a Google Ads manager (MCC) account."
	Summary      = "List accounts under a manager account"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+folder-tree"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

// customer_client is the manager's view of everything below it, at any depth.
// level 0 is the manager itself, so the default filter drops it and returns the
// accounts someone would actually want to act on.
const clientFields = "customer_client.id, customer_client.descriptive_name, customer_client.currency_code, " +
	"customer_client.time_zone, customer_client.manager, customer_client.level, customer_client.status, " +
	"customer_client.test_account, customer_client.client_customer"

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID to list beneath", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "include_manager", Type: core.ConnectionTypeBoolean, Label: "Include the manager account itself, and any managers below it"},
	{Name: "include_hidden", Type: core.ConnectionTypeBoolean, Label: "Include cancelled and suspended accounts"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum accounts to return", Placeholder: "200"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Client Accounts"},
	{Name: "table", Type: core.ConnectionTypeObject, Label: "Client Accounts (flattened)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, customerID, login, err := gads.GetAuth(inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	var conditions []string
	if !gads.OptionalBool("include_manager", inputs) {
		conditions = append(conditions, "customer_client.manager = FALSE")
	}
	if !gads.OptionalBool("include_hidden", inputs) {
		conditions = append(conditions, "customer_client.status = 'ENABLED'")
	}

	limit := gads.OptionalInt("limit", inputs)
	if limit == nil {
		fallback := int64(200)
		limit = &fallback
	}

	query := gads.BuildQuery([]string{clientFields}, "customer_client", conditions, "customer_client.level, customer_client.descriptive_name", limit)

	// Listing beneath a manager requires the request to be MADE as that
	// manager, not merely to name it — so when no login is set explicitly the
	// account being listed is itself the manager.
	if login == "" {
		login = customerID
	}

	rows, err := gads.NewClient(token, login).Search(flow, customerID, query)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	return gads.RowsResult(rows, fmt.Sprintf("Found %d account(s) beneath manager %s", len(rows), customerID), nil), nil
}

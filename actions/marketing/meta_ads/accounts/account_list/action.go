// Package account_list lists the Meta ad accounts the connected token can reach.
package account_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Ad Accounts: List"
	Description  = "List the Meta ad accounts this access token can manage."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+list"
	Date         = "17/08/2026"
	Type         = core.ActionTypeAction
)

// defaultFields includes currency and account_status deliberately: currency is
// what every budget conversion depends on, and account_status is the difference
// between "this account works" and "this account is disabled and every write
// will fail for a reason the error will not make obvious".
const defaultFields = "id,account_id,name,currency,account_status,timezone_name,amount_spent,balance"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: defaultFields},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "25"},
	{Name: "after", Type: core.ConnectionTypeString, Label: "Page Cursor (from a previous run)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Ad Accounts"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "next_cursor", Type: core.ConnectionTypeString, Label: "Next Page Cursor"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, secret, err := meta.GetAuth(inputs)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	p := url.Values{"fields": {meta.Fields("fields", inputs, defaultFields)}}
	meta.SetParam(p, "limit", "limit", inputs)
	meta.SetParam(p, "after", "after", inputs)

	resp, err := meta.NewClient(token, secret).Get(flow, "/me/adaccounts", p)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	accounts := meta.Data(resp)
	next := meta.NextCursor(resp)
	summary := fmt.Sprintf("Found %d ad account(s)", len(accounts))
	if next != "" {
		summary += " (more pages available — pass next_cursor back in as the Page Cursor)"
	}
	return meta.ListResult(accounts, summary, map[string]interface{}{"next_cursor": next}), nil
}

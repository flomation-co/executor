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
	Description  = "List the Meta ad accounts this access token can manage, or those a business owns."
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
	// Optional, and the fastest way to tell an empty result apart from a
	// misconfigured one: /me/adaccounts answers "what is assigned to this
	// token", whereas a business's owned_ad_accounts answers "what exists at
	// all". Those two questions have very different fixes.
	{Name: "business_id", Type: core.ConnectionTypeString, Label: "Business ID (optional — list accounts the BUSINESS owns instead)", Placeholder: "1507486194117794"},
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

	businessID := meta.OptionalString("business_id", inputs)
	path := "/me/adaccounts"
	if businessID != "" {
		path = "/" + businessID + "/owned_ad_accounts"
	}

	resp, err := meta.NewClient(token, secret).Get(flow, path, p)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	accounts := meta.Data(resp)
	next := meta.NextCursor(resp)

	summary := fmt.Sprintf("Found %d ad account(s)", len(accounts))
	if len(accounts) == 0 {
		summary += ". " + EmptyExplanation(businessID)
	}
	if next != "" {
		summary += " (more pages available — pass next_cursor back in as the Page Cursor)"
	}
	return meta.ListResult(accounts, summary, map[string]interface{}{"next_cursor": next}), nil
}

// EmptyExplanation names the likely cause of an empty result.
//
// An empty list here is a 200 with no data, which is indistinguishable from a
// broken integration unless the action says otherwise — and it has one
// overwhelmingly common cause. /me/adaccounts returns the accounts assigned to
// THIS TOKEN's user, so a System User token whose ad account was never assigned
// returns nothing at all, successfully. Left unexplained, that sends the reader
// looking at scopes and tokens, which are the two things that are usually fine.
func EmptyExplanation(businessID string) string {
	if businessID != "" {
		return "This business portfolio owns no ad accounts. Create one in Ads Manager (adsmanager.facebook.com), " +
			"or if the accounts belong to a client rather than this business, they will not appear here — owned and client accounts are separate lists."
	}
	return "The token is valid, so this is almost always an ASSIGNMENT problem rather than a scope one: " +
		"/me/adaccounts returns only the ad accounts assigned to this token's user. " +
		"For a System User token, fix it in Business Settings > Users > System users > (your user) > Assign Assets > Ad accounts, " +
		"and make sure a permission level such as 'Manage campaigns' is ticked — attaching the asset without ticking one leaves it present but unusable. " +
		"To check whether any ad account exists at all, re-run this action with the Business ID set."
}

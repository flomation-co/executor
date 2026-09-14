// Package campaign_status_set enables, pauses or removes a Google Ads campaign.
package campaign_status_set

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Campaign: Set Status"
	Description  = "Enable, pause or remove a Google Ads campaign. Removing is permanent."
	Summary      = "Pause or enable a Google Ads campaign"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+pause"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

// Its own action rather than a corner of Campaign: Update, because pausing is
// by far the most common thing an automation does to a campaign — an overspend
// guard, an out-of-hours rule, a stock-out — and because it spares the caller
// (often an agent) from having to construct an update mask correctly to do the
// single most consequential operation in the integration.
var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Required: true, Options: []core.ConnectionOption{{Name: "Pause", Value: "PAUSED"}, {Name: "Enable", Value: "ENABLED"}, {Name: "Remove (permanent)", Value: "REMOVED"}}},
	{Name: "validate_only", Type: core.ConnectionTypeBoolean, Label: "Dry run — validate without changing anything"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign ID"},
	{Name: "resource_name", Type: core.ConnectionTypeString, Label: "Campaign Resource Name"},
	{Name: "resource_names", Type: core.ConnectionTypeObject, Label: "Changed Resource Names"},
	{Name: "dry_run", Type: core.ConnectionTypeBoolean, Label: "Was a Dry Run"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, customerID, login, err := gads.GetAuth(inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	campaignID, err := gads.RequiredString("campaign_id", inputs)
	if err != nil {
		return gads.ErrorResult("a campaign ID is required"), nil
	}

	status := strings.ToUpper(gads.OptionalString("status", inputs))
	switch status {
	case "ENABLED", "PAUSED", "REMOVED":
	case "":
		return gads.ErrorResult("a status is required (Enable, Pause or Remove)"), nil
	default:
		// ACTIVE is Meta's vocabulary and a natural thing for someone — or an
		// agent that has used the Meta Ads actions — to reach for.
		if status == "ACTIVE" {
			return gads.ErrorResult("Google Ads uses ENABLED rather than ACTIVE — set the status to Enable"), nil
		}
		return gads.ErrorResult(fmt.Sprintf("%q is not a campaign status; use ENABLED, PAUSED or REMOVED", status)), nil
	}

	operations := []interface{}{map[string]interface{}{
		"update": map[string]interface{}{
			"resourceName": fmt.Sprintf("customers/%s/campaigns/%s", customerID, campaignID),
			"status":       status,
		},
		"updateMask": "status",
	}}
	opt := gads.MutateOptions{ValidateOnly: gads.OptionalBool("validate_only", inputs), PartialFailure: false}

	resp, err := gads.NewClient(token, login).Mutate(flow, customerID, "campaigns", operations, opt)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Set campaign %s to %s", campaignID, status)
	if status == "REMOVED" {
		// REMOVED is terminal in Google Ads — there is no un-remove — so say so
		// in the result rather than only in the input label.
		summary += " (removal is permanent and cannot be undone)"
	}
	return gads.MutateResult(resp, opt, summary, nil), nil
}

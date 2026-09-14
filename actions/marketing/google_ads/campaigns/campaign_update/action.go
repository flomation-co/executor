// Package campaign_update changes settings on an existing Google Ads campaign.
package campaign_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Campaign: Update"
	Description  = "Change a Google Ads campaign's name, dates, bidding strategy or tracking settings."
	Summary      = "Update a Google Ads campaign"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+pencil"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "New Campaign Name"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{{Name: "Leave unchanged", Value: ""}, {Name: "Enabled", Value: "ENABLED"}, {Name: "Paused", Value: "PAUSED"}}},
	{Name: "bidding_strategy", Type: core.ConnectionTypeString, Label: "Bidding Strategy", Options: []core.ConnectionOption{{Name: "Leave unchanged", Value: ""}, {Name: "Maximise clicks", Value: "TARGET_SPEND"}, {Name: "Manual CPC", Value: "MANUAL_CPC"}, {Name: "Maximise conversions", Value: "MAXIMIZE_CONVERSIONS"}, {Name: "Maximise conversion value", Value: "MAXIMIZE_CONVERSION_VALUE"}, {Name: "Target CPA", Value: "TARGET_CPA"}, {Name: "Target ROAS", Value: "TARGET_ROAS"}}},
	{Name: "target_cpa", Type: core.ConnectionTypeMoney, Label: "Target CPA (Target CPA and Maximise conversions only)"},
	{Name: "target_roas", Type: core.ConnectionTypeString, Label: "Target ROAS (e.g. 4 for a 400% return)"},
	{Name: "enhanced_cpc", Type: core.ConnectionTypeBoolean, Label: "Enhanced CPC (Manual CPC only)"},
	{Name: "start_date", Type: core.ConnectionTypeString, Label: "Start Date (YYYY-MM-DD, account's timezone)"},
	{Name: "end_date", Type: core.ConnectionTypeString, Label: "End Date (YYYY-MM-DD, account's timezone)"},
	{Name: "tracking_url_template", Type: core.ConnectionTypeString, Label: "Tracking URL Template"},
	{Name: "final_url_suffix", Type: core.ConnectionTypeString, Label: "Final URL Suffix"},
	{Name: "validate_only", Type: core.ConnectionTypeBoolean, Label: "Dry run — validate without changing anything"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign ID"},
	{Name: "resource_name", Type: core.ConnectionTypeString, Label: "Campaign Resource Name"},
	{Name: "resource_names", Type: core.ConnectionTypeObject, Label: "Changed Resource Names"},
	{Name: "updated_fields", Type: core.ConnectionTypeObject, Label: "Fields Changed"},
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

	campaign := map[string]interface{}{
		"resourceName": fmt.Sprintf("customers/%s/campaigns/%s", customerID, campaignID),
	}
	// updateMask is the whole game on a Google Ads update: the server changes
	// ONLY the fields the mask names, so a field set in the payload but absent
	// from the mask is silently ignored — the call succeeds and nothing happens.
	// Building the two together, field by field, is what stops them drifting.
	var mask []string

	if v := gads.OptionalString("name", inputs); v != "" {
		campaign["name"] = v
		mask = append(mask, "name")
	}
	if v := strings.ToUpper(gads.OptionalString("status", inputs)); v != "" {
		campaign["status"] = v
		mask = append(mask, "status")
	}
	if v := gads.OptionalString("tracking_url_template", inputs); v != "" {
		campaign["trackingUrlTemplate"] = v
		mask = append(mask, "tracking_url_template")
	}
	if v := gads.OptionalString("final_url_suffix", inputs); v != "" {
		campaign["finalUrlSuffix"] = v
		mask = append(mask, "final_url_suffix")
	}

	start, err := gads.CampaignDateTime(gads.OptionalString("start_date", inputs), false)
	if err != nil {
		return gads.ErrorResult("start date: " + err.Error()), nil
	}
	if start != "" {
		campaign["startDateTime"] = start
		mask = append(mask, "start_date_time")
	}
	end, err := gads.CampaignDateTime(gads.OptionalString("end_date", inputs), true)
	if err != nil {
		return gads.ErrorResult("end date: " + err.Error()), nil
	}
	if end != "" {
		campaign["endDateTime"] = end
		mask = append(mask, "end_date_time")
	}

	bidPaths, err := gads.ApplyBiddingStrategy(gads.OptionalString("bidding_strategy", inputs), inputs, campaign)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	mask = append(mask, bidPaths...)

	if len(mask) == 0 {
		return gads.ErrorResult("nothing to update — set at least one field to change"), nil
	}

	operations := []interface{}{map[string]interface{}{
		"update":     campaign,
		"updateMask": strings.Join(mask, ","),
	}}
	opt := gads.MutateOptions{ValidateOnly: gads.OptionalBool("validate_only", inputs), PartialFailure: false}

	resp, err := gads.NewClient(token, login).Mutate(flow, customerID, "campaigns", operations, opt)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	return gads.MutateResult(resp, opt,
		fmt.Sprintf("Updated campaign %s (%s)", campaignID, strings.Join(mask, ", ")),
		map[string]interface{}{"updated_fields": mask}), nil
}

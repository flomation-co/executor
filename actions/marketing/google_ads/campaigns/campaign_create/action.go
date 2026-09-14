// Package campaign_create creates a Google Ads campaign together with its
// budget, in one atomic request.
package campaign_create

import (
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Campaign: Create"
	Description  = "Create a Google Ads campaign and its daily budget in one atomic request. Starts paused."
	Summary      = "Create a Google Ads campaign"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+plus"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

// The temporary resource name that lets the campaign reference a budget which
// does not exist yet. Within one googleAds:mutate a negative id is a
// placeholder the server substitutes once everything validates, so the budget
// and the campaign are created together or not at all.
const budgetTempID = -1

// Campaign types that need an asset group, a listing group filter or a linked
// merchant/app account before they can serve. Creating one from these inputs
// alone would always fail, so they are refused with an explanation rather than
// offered and then rejected by Google.
var unsupportedChannels = map[string]string{
	"PERFORMANCE_MAX": "Performance Max campaigns need asset groups, which this action does not create yet",
	"SHOPPING":        "Shopping campaigns need a linked Merchant Center account and a listing group",
	"VIDEO":           "Video campaigns need a linked YouTube video asset",
	"DEMAND_GEN":      "Demand Gen campaigns need asset groups",
	"MULTI_CHANNEL":   "App campaigns need a linked mobile app",
	"LOCAL_SERVICES":  "Local Services campaigns are managed from the Local Services app, not the API",
}

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Campaign Name (must be unique in the account)", Required: true},
	{Name: "channel_type", Type: core.ConnectionTypeString, Label: "Campaign Type", Required: true, Options: []core.ConnectionOption{{Name: "Search", Value: "SEARCH"}, {Name: "Display", Value: "DISPLAY"}}},
	{Name: "daily_budget", Type: core.ConnectionTypeMoney, Label: "Daily Budget (in the account's currency)", Required: true},
	{Name: "bidding_strategy", Type: core.ConnectionTypeString, Label: "Bidding Strategy", Required: true, Options: []core.ConnectionOption{{Name: "Maximise clicks", Value: "TARGET_SPEND"}, {Name: "Manual CPC", Value: "MANUAL_CPC"}, {Name: "Maximise conversions", Value: "MAXIMIZE_CONVERSIONS"}, {Name: "Maximise conversion value", Value: "MAXIMIZE_CONVERSION_VALUE"}, {Name: "Target CPA", Value: "TARGET_CPA"}, {Name: "Target ROAS", Value: "TARGET_ROAS"}}},
	{Name: "target_cpa", Type: core.ConnectionTypeMoney, Label: "Target CPA (Target CPA and Maximise conversions only)"},
	{Name: "target_roas", Type: core.ConnectionTypeString, Label: "Target ROAS (e.g. 4 for a 400% return)"},
	{Name: "enhanced_cpc", Type: core.ConnectionTypeBoolean, Label: "Enhanced CPC (Manual CPC only)"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status on creation", Options: []core.ConnectionOption{{Name: "Paused (recommended)", Value: "PAUSED"}, {Name: "Enabled — starts spending immediately", Value: "ENABLED"}}},
	{Name: "start_date", Type: core.ConnectionTypeString, Label: "Start Date (YYYY-MM-DD, account's timezone)"},
	{Name: "end_date", Type: core.ConnectionTypeString, Label: "End Date (YYYY-MM-DD, account's timezone)"},
	{Name: "target_search_network", Type: core.ConnectionTypeBoolean, Label: "Include Google search partners (Search campaigns)"},
	{Name: "target_content_network", Type: core.ConnectionTypeBoolean, Label: "Include the Display Network (Search campaigns)"},
	{Name: "budget_name", Type: core.ConnectionTypeString, Label: "Budget Name (defaults to the campaign name)"},
	{Name: "validate_only", Type: core.ConnectionTypeBoolean, Label: "Dry run — validate without creating anything"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign ID"},
	{Name: "resource_name", Type: core.ConnectionTypeString, Label: "Campaign Resource Name"},
	{Name: "budget_resource_name", Type: core.ConnectionTypeString, Label: "Budget Resource Name"},
	{Name: "resource_names", Type: core.ConnectionTypeObject, Label: "All Created Resource Names"},
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

	name, err := gads.RequiredString("name", inputs)
	if err != nil {
		return gads.ErrorResult("a campaign name is required"), nil
	}

	channel := strings.ToUpper(gads.OptionalString("channel_type", inputs))
	if channel == "" {
		return gads.ErrorResult("a campaign type is required (Search or Display)"), nil
	}
	if reason, blocked := unsupportedChannels[channel]; blocked {
		return gads.ErrorResult(fmt.Sprintf("%s campaigns cannot be created here: %s", channel, reason)), nil
	}

	budgetMicros, err := gads.MoneyToMicros("daily_budget", inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	if budgetMicros == nil || *budgetMicros <= 0 {
		return gads.ErrorResult("a daily budget greater than zero is required"), nil
	}

	budgetName := gads.OptionalString("budget_name", inputs)
	if budgetName == "" {
		budgetName = name + " budget"
	}

	budgetResource := fmt.Sprintf("customers/%s/campaignBudgets/%d", customerID, budgetTempID)
	budget := map[string]interface{}{
		"resourceName":   budgetResource,
		"name":           budgetName,
		"amountMicros":   strconv.FormatInt(*budgetMicros, 10),
		"deliveryMethod": "STANDARD",
		// Not shared: the budget belongs to this campaign alone. A shared budget
		// spreads spend across campaigns, which is rarely what someone creating
		// a campaign and a budget in one breath means, and it makes the budget
		// name globally unique-or-reject.
		"explicitlyShared": false,
	}

	campaign := map[string]interface{}{
		"name":                   name,
		"advertisingChannelType": channel,
		"campaignBudget":         budgetResource,
	}

	// Default PAUSED, against Google's own default of ENABLED. A campaign
	// created by an automation — very possibly by an agent — should not start
	// spending money the moment it exists; enabling it is one deliberate call
	// to Campaign: Set Status away.
	status := strings.ToUpper(gads.OptionalString("status", inputs))
	if status == "" {
		status = "PAUSED"
	}
	campaign["status"] = status

	if _, err := gads.ApplyBiddingStrategy(gads.OptionalString("bidding_strategy", inputs), inputs, campaign); err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	start, err := gads.CampaignDateTime(gads.OptionalString("start_date", inputs), false)
	if err != nil {
		return gads.ErrorResult("start date: " + err.Error()), nil
	}
	if start != "" {
		campaign["startDateTime"] = start
	}
	end, err := gads.CampaignDateTime(gads.OptionalString("end_date", inputs), true)
	if err != nil {
		return gads.ErrorResult("end date: " + err.Error()), nil
	}
	if end != "" {
		campaign["endDateTime"] = end
	}

	// Network settings apply to Search and Display only; sending them for other
	// channel types is rejected.
	switch channel {
	case "SEARCH":
		campaign["networkSettings"] = map[string]interface{}{
			"targetGoogleSearch":   true,
			"targetSearchNetwork":  gads.OptionalBool("target_search_network", inputs),
			"targetContentNetwork": gads.OptionalBool("target_content_network", inputs),
		}
	case "DISPLAY":
		campaign["networkSettings"] = map[string]interface{}{
			"targetGoogleSearch":   false,
			"targetSearchNetwork":  false,
			"targetContentNetwork": true,
		}
	}

	operations := []interface{}{
		map[string]interface{}{"campaignBudgetOperation": map[string]interface{}{"create": budget}},
		map[string]interface{}{"campaignOperation": map[string]interface{}{"create": campaign}},
	}

	// partial_failure is deliberately NOT offered here and is forced off: a
	// campaign whose budget failed to create is not a partial success, it is a
	// broken account state that has to be cleaned up by hand.
	opt := gads.MutateOptions{ValidateOnly: gads.OptionalBool("validate_only", inputs), PartialFailure: false}

	resp, err := gads.NewClient(token, login).MutateAtomic(flow, customerID, operations, opt)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	names := gads.MutatedResourceNames(resp)
	result := gads.MutateResult(resp, opt, fmt.Sprintf("Created campaign %q (%s, %s)", name, channel, status), nil)

	// The atomic mutate returns results in operation order, so the budget is
	// first and the campaign second. Re-point the headline id at the CAMPAIGN,
	// which is what a downstream node wants to wire into.
	if len(names) >= 2 {
		result["budget_resource_name"] = names[0]
		result["resource_name"] = names[1]
		result["id"] = gads.ResourceID(names[1])
	}
	return result, nil
}

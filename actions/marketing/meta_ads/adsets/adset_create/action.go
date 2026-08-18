// Package adset_create creates a Meta ad set — the level that carries
// targeting, budget, schedule and bidding.
package adset_create

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Ad Sets: Create"
	Description  = "Create a Meta ad set with targeting, budget, schedule and optimisation goal."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+plus"
	Date         = "17/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Ad Account ID", Placeholder: "act_1234567890 or 1234567890", Required: true},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Ad Set Name", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{
		{Name: "Paused (default — safe)", Value: "PAUSED"},
		{Name: "Active — starts spending immediately", Value: "ACTIVE"},
	}},
	{Name: "daily_budget", Type: core.ConnectionTypeMoney, Label: "Daily Budget in POUNDS/major units — e.g. 10.00 means £10.00. Do NOT pre-convert to pence; the action does that.", Placeholder: "50.00"},
	{Name: "lifetime_budget", Type: core.ConnectionTypeMoney, Label: "Lifetime Budget in POUNDS/major units — e.g. 500.00 means £500.00. Do NOT pre-convert to pence. Needs an End Time.", Placeholder: "500.00"},
	{Name: "billing_event", Type: core.ConnectionTypeString, Label: "Billing Event", Options: []core.ConnectionOption{
		{Name: "Impressions", Value: "IMPRESSIONS"},
		{Name: "Link clicks", Value: "LINK_CLICKS"},
		{Name: "Post engagement", Value: "POST_ENGAGEMENT"},
		{Name: "Thruplay (video)", Value: "THRUPLAY"},
	}},
	{Name: "optimization_goal", Type: core.ConnectionTypeString, Label: "Optimisation Goal", Options: []core.ConnectionOption{
		{Name: "Link clicks", Value: "LINK_CLICKS"},
		{Name: "Landing page views", Value: "LANDING_PAGE_VIEWS"},
		{Name: "Impressions", Value: "IMPRESSIONS"},
		{Name: "Reach", Value: "REACH"},
		{Name: "Leads", Value: "LEAD_GENERATION"},
		{Name: "Conversions (offsite)", Value: "OFFSITE_CONVERSIONS"},
		{Name: "Post engagement", Value: "POST_ENGAGEMENT"},
		{Name: "Thruplay (video)", Value: "THRUPLAY"},
	}},
	// bid_strategy was previously reachable only by hand-assembling it into the
	// Additional Fields JSON, which is exactly where an agent goes wrong — it is
	// a small fixed enum, so it belongs here with its valid values.
	//
	// Note this is an AD SET level setting. Under campaign budget optimisation
	// the CAMPAIGN owns the bid strategy and this is ignored.
	{Name: "bid_strategy", Type: core.ConnectionTypeString, Label: "Bid Strategy", Options: []core.ConnectionOption{
		{Name: "Lowest cost (automatic — no cap needed)", Value: "LOWEST_COST_WITHOUT_CAP"},
		{Name: "Lowest cost with bid cap (needs a Bid Cap)", Value: "LOWEST_COST_WITH_BID_CAP"},
		{Name: "Cost cap (needs a Bid Cap)", Value: "COST_CAP"},
	}},
	{Name: "bid_amount", Type: core.ConnectionTypeMoney, Label: "Bid Cap in POUNDS/major units — e.g. 1.00 means £1.00. Do NOT pre-convert to pence.", Placeholder: "2.00"},
	{Name: "start_time", Type: core.ConnectionTypeString, Label: "Start Time (ISO 8601)", Placeholder: "2026-09-01T09:00:00+0100"},
	{Name: "end_time", Type: core.ConnectionTypeString, Label: "End Time (ISO 8601)", Placeholder: "2026-09-30T23:59:00+0100"},
	// Targeting is a large, deeply nested structure with hundreds of valid
	// shapes, so it is taken as JSON rather than pretended to be a handful of
	// dropdowns. The placeholder is a working minimal example.
	{Name: "targeting", Type: core.ConnectionTypeText, Label: "Targeting (JSON object)", Placeholder: `{"geo_locations":{"countries":["GB"]},"age_min":25,"age_max":54}`},
	{Name: "promoted_object", Type: core.ConnectionTypeText, Label: "Promoted Object (JSON, required by some goals)", Placeholder: `{"page_id":"1234567890"}`},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON object)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ad Set ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "currency", Type: core.ConnectionTypeString, Label: "Ad Account Currency"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, secret, err := meta.GetAuth(inputs)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}
	account, err := meta.RequiredString("account_id", inputs)
	if err != nil {
		return meta.ErrorResult("an ad account ID is required"), nil
	}
	campaign, err := meta.RequiredString("campaign_id", inputs)
	if err != nil {
		return meta.ErrorResult("a campaign ID is required — an ad set must belong to a campaign"), nil
	}
	name, err := meta.RequiredString("name", inputs)
	if err != nil {
		return meta.ErrorResult("an ad set name is required"), nil
	}

	client := meta.NewClient(token, secret)
	p := url.Values{"name": {name}, "campaign_id": {campaign}}

	status := meta.OptionalString("status", inputs)
	if status == "" {
		status = "PAUSED"
	}
	p.Set("status", status)

	meta.SetParam(p, "billing_event", "billing_event", inputs)
	meta.SetParam(p, "optimization_goal", "optimization_goal", inputs)
	meta.SetParam(p, "bid_strategy", "bid_strategy", inputs)

	// Bid/billing combinations Meta rejects, caught here because its own error
	// does not name the offending pair — it complains about a missing
	// bid_amount without saying which of the three settings made it mandatory,
	// which sends the reader round several wrong fixes first.
	if msg := bidRequirementError(
		meta.OptionalString("optimization_goal", inputs),
		meta.OptionalString("billing_event", inputs),
		meta.OptionalString("bid_strategy", inputs),
		meta.OptionalString("bid_amount", inputs),
	); msg != "" {
		return meta.ErrorResult(msg), nil
	}
	meta.SetParam(p, "start_time", "start_time", inputs)
	meta.SetParam(p, "end_time", "end_time", inputs)

	for _, f := range []struct{ key, input string }{{"targeting", "targeting"}, {"promoted_object", "promoted_object"}} {
		if err := meta.SetJSONParam(p, f.key, f.input, inputs); err != nil {
			return meta.ErrorResult(err.Error()), nil
		}
	}

	daily := meta.OptionalString("daily_budget", inputs)
	lifetime := meta.OptionalString("lifetime_budget", inputs)

	// Meta only reports the campaign-vs-ad-set budget conflict after the
	// attempt, and never says which side already has a budget. One read of the
	// campaign answers it — and picks up a cap-requiring bid strategy at the
	// same time, since the two constraints travel together and are far easier
	// to fix in one pass than discovered one failure at a time.
	if msg := meta.CampaignBudgetConflict(flow, client, campaign, daily != "" || lifetime != ""); msg != "" {
		return meta.ErrorResult(msg), nil
	}

	// Meta rejects a lifetime budget without an end time, with a message that
	// does not name the missing field. Catching it here costs one comparison and
	// saves a confusing round trip.
	if lifetime != "" && meta.OptionalString("end_time", inputs) == "" {
		return meta.ErrorResult("a lifetime budget requires an End Time — Meta cannot pace a lifetime budget without knowing when it ends"), nil
	}

	currency := ""
	if daily != "" || lifetime != "" || meta.OptionalString("bid_amount", inputs) != "" {
		currency, err = meta.AccountCurrency(flow, client, account)
		if err != nil {
			return meta.ErrorResult(err.Error()), nil
		}
		for _, field := range []string{"daily_budget", "lifetime_budget", "bid_amount"} {
			minor, cerr := meta.BudgetMinorUnits(field, currency, inputs)
			if cerr != nil {
				return meta.ErrorResult(fmt.Sprintf("%s: %s", field, cerr.Error())), nil
			}
			if minor != nil {
				p.Set(field, strconv.FormatInt(*minor, 10))
			}
		}
	}

	if err := meta.MergeJSONFields(p, "fields", inputs); err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	resp, err := client.Post(flow, meta.AccountPath(account)+"/adsets", p)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	id, _ := resp["id"].(string)
	summary := fmt.Sprintf("Created ad set %q (%s) under campaign %s, status %s", name, id, campaign, status)
	if status == "PAUSED" {
		summary += ". It is PAUSED and will not spend until set to ACTIVE."
	}
	if currency != "" {
		summary += fmt.Sprintf(" Budget interpreted in %s.", currency)
	}
	return meta.OkResult(summary, map[string]interface{}{"id": id, "status": status, "currency": currency}), nil
}

// bidRequirementError returns a message when the chosen bid settings need a bid
// cap that has not been given, or "" when the combination is valid.
//
// Two separate rules, both of which surface from Meta as the same unhelpful
// complaint about a missing bid_amount:
//
//  1. LOWEST_COST_WITH_BID_CAP and COST_CAP require a cap by definition. This
//     one is documented.
//  2. LINK_CLICKS optimisation billed on IMPRESSIONS also requires one, even
//     with no bid strategy set and even when the budget lives on the campaign.
//     That pairing is NOT obvious from the reference, and it is the one that
//     cost two round trips and a wrong diagnosis to find: Meta objects to the
//     bid while the reader is looking at the budget.
func bidRequirementError(optimisationGoal, billingEvent, bidStrategy, bidAmount string) string {
	if strings.TrimSpace(bidAmount) != "" {
		return ""
	}

	switch bidStrategy {
	case "LOWEST_COST_WITH_BID_CAP", "COST_CAP":
		return "Bid Strategy " + bidStrategy + " requires a Bid Cap. Set one, or choose LOWEST_COST_WITHOUT_CAP, which bids automatically and needs no cap."
	}

	if optimisationGoal == "LINK_CLICKS" && billingEvent == "IMPRESSIONS" {
		return "Meta requires a Bid Cap when LINK_CLICKS optimisation is billed on IMPRESSIONS. " +
			"Set a Bid Cap, or change the Billing Event. " +
			"Note the billing event decides what you are CHARGED for: IMPRESSIONS bills per 1,000 impressions, LINK_CLICKS bills per click — it is not a formality. " +
			"Moving the budget to the campaign does NOT lift this requirement. " +
			"If a Bid Cap is still demanded after changing the billing event, check the PARENT CAMPAIGN's bid_strategy: " +
			"under campaign budget optimisation the campaign owns the bid strategy, and a cap-requiring one there (LOWEST_COST_WITH_BID_CAP or COST_CAP) forces a Bid Cap on every ad set beneath it, whatever this ad set is set to. " +
			"Read it with Campaigns: List, requesting the bid_strategy field."
	}
	return ""
}

// Package campaign_create creates a Meta ad campaign.
package campaign_create

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
	Name         = "Campaigns: Create"
	Description  = "Create a Meta ad campaign with an objective and optional budget."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+plus"
	Date         = "17/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Ad Account ID", Placeholder: "act_1234567890 or 1234567890", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Campaign Name", Required: true},
	// The OUTCOME_* objectives are Meta's current set; the older LEAD_GENERATION
	// / CONVERSIONS names were retired and are rejected outright.
	{Name: "objective", Type: core.ConnectionTypeString, Label: "Objective", Required: true, Options: []core.ConnectionOption{
		{Name: "Awareness", Value: "OUTCOME_AWARENESS"},
		{Name: "Traffic", Value: "OUTCOME_TRAFFIC"},
		{Name: "Engagement", Value: "OUTCOME_ENGAGEMENT"},
		{Name: "Leads", Value: "OUTCOME_LEADS"},
		{Name: "App Promotion", Value: "OUTCOME_APP_PROMOTION"},
		{Name: "Sales", Value: "OUTCOME_SALES"},
	}},
	// Defaults to PAUSED. Creating a campaign that immediately starts spending
	// is not a reasonable default for an automated action — going live should be
	// a deliberate second step (Campaigns: Update), not a side effect of
	// leaving a field alone.
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{
		{Name: "Paused (default — safe)", Value: "PAUSED"},
		{Name: "Active — starts spending immediately", Value: "ACTIVE"},
	}},
	{Name: "daily_budget", Type: core.ConnectionTypeMoney, Label: "Daily Budget in POUNDS/major units — e.g. 10.00 means £10.00. Do NOT pre-convert to pence; the action does that.", Placeholder: "50.00"},
	{Name: "lifetime_budget", Type: core.ConnectionTypeMoney, Label: "Lifetime Budget in POUNDS/major units — e.g. 500.00 means £500.00. Do NOT pre-convert to pence; the action does that.", Placeholder: "500.00"},
	// Exposed here as well as on the ad set because THIS is the level that
	// bit: under campaign budget optimisation the campaign owns the bid
	// strategy, and a cap-requiring one set here forces a Bid Cap on every ad
	// set beneath it — with an error that points at the ad set, not here.
	// Previously reachable only through the Additional Fields JSON, which is
	// how it got set unintentionally in the first place.
	{Name: "bid_strategy", Type: core.ConnectionTypeString, Label: "Bid Strategy", Options: []core.ConnectionOption{
		{Name: "Lowest cost (automatic — no cap needed)", Value: "LOWEST_COST_WITHOUT_CAP"},
		{Name: "Lowest cost with bid cap (forces a Bid Cap on every ad set)", Value: "LOWEST_COST_WITH_BID_CAP"},
		{Name: "Cost cap (forces a Bid Cap on every ad set)", Value: "COST_CAP"},
	}},
	{Name: "special_ad_categories", Type: core.ConnectionTypeString, Label: "Special Ad Categories", Placeholder: "NONE, HOUSING, EMPLOYMENT, CREDIT, ISSUES_ELECTIONS_POLITICS, ONLINE_GAMBLING_AND_GAMING", Options: []core.ConnectionOption{
		{Name: "None", Value: "NONE"},
		{Name: "Housing", Value: "HOUSING"},
		{Name: "Employment", Value: "EMPLOYMENT"},
		{Name: "Credit", Value: "CREDIT"},
		{Name: "Social issues, elections or politics", Value: "ISSUES_ELECTIONS_POLITICS"},
		{Name: "Online gambling and gaming", Value: "ONLINE_GAMBLING_AND_GAMING"},
	}},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON object)", Placeholder: `{"bid_strategy":"LOWEST_COST_WITHOUT_CAP"}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign ID"},
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
	name, err := meta.RequiredString("name", inputs)
	if err != nil {
		return meta.ErrorResult("a campaign name is required"), nil
	}
	objective, err := meta.RequiredString("objective", inputs)
	if err != nil {
		return meta.ErrorResult("an objective is required"), nil
	}

	client := meta.NewClient(token, secret)

	p := url.Values{"name": {name}, "objective": {objective}}

	status := meta.OptionalString("status", inputs)
	if status == "" {
		status = "PAUSED"
	}
	p.Set("status", status)
	meta.SetParam(p, "bid_strategy", "bid_strategy", inputs)

	// special_ad_categories is REQUIRED by Meta on every campaign create — an
	// omitted value is rejected rather than defaulted. NONE is the honest
	// default, and it must go as a JSON array.
	category := meta.OptionalString("special_ad_categories", inputs)
	if category == "" {
		category = "NONE"
	}
	p.Set("special_ad_categories", `["`+category+`"]`)

	// Budgets are minor-unit integers in the ACCOUNT's currency, so resolve the
	// currency from the account rather than assuming. Only fetched when a budget
	// is actually being set, to avoid an extra call on every create.
	currency := ""
	var budgetNotes []string
	if meta.OptionalString("daily_budget", inputs) != "" || meta.OptionalString("lifetime_budget", inputs) != "" {
		currency, err = meta.AccountCurrency(flow, client, account)
		if err != nil {
			return meta.ErrorResult(err.Error()), nil
		}
		for _, field := range []string{"daily_budget", "lifetime_budget"} {
			minor, cerr := meta.BudgetMinorUnits(field, currency, inputs)
			if cerr != nil {
				return meta.ErrorResult(fmt.Sprintf("%s: %s", field, cerr.Error())), nil
			}
			if minor != nil {
				p.Set(field, strconv.FormatInt(*minor, 10))
				// Echo exactly what was sent — a hundredfold conversion error
				// is otherwise invisible until the money has gone.
				budgetNotes = append(budgetNotes,
					meta.DescribeBudget(strings.ReplaceAll(field, "_", " "), meta.OptionalString(field, inputs), currency, *minor))
			}
		}
	}

	// Applied last so an explicit override beats the curated inputs above.
	if err := meta.MergeJSONFields(p, "fields", inputs); err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	resp, err := client.Post(flow, meta.AccountPath(account)+"/campaigns", p)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	id, _ := resp["id"].(string)
	summary := fmt.Sprintf("Created campaign %q (%s) with objective %s, status %s", name, id, objective, status)
	if status == "PAUSED" {
		summary += ". It is PAUSED and will not spend until set to ACTIVE."
	}
	if len(budgetNotes) > 0 {
		summary += ". " + strings.Join(budgetNotes, "; ") + "."
	}
	return meta.OkResult(summary, map[string]interface{}{
		"id":       id,
		"status":   status,
		"currency": currency,
	}), nil
}

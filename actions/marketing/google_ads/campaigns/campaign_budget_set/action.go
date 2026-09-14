// Package campaign_budget_set changes the daily amount on a campaign budget.
package campaign_budget_set

import (
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Campaign: Set Budget"
	Description  = "Change a Google Ads campaign's daily budget, by campaign ID or budget ID."
	Summary      = "Change a Google Ads campaign budget"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+gauge"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID (its budget is looked up for you)"},
	{Name: "budget_id", Type: core.ConnectionTypeString, Label: "Budget ID (use instead of a Campaign ID)"},
	{Name: "daily_budget", Type: core.ConnectionTypeMoney, Label: "New Daily Budget (in the account's currency)", Required: true},
	{Name: "validate_only", Type: core.ConnectionTypeBoolean, Label: "Dry run — validate without changing anything"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Budget ID"},
	{Name: "resource_name", Type: core.ConnectionTypeString, Label: "Budget Resource Name"},
	{Name: "resource_names", Type: core.ConnectionTypeObject, Label: "Changed Resource Names"},
	{Name: "shared_with_campaigns", Type: core.ConnectionTypeInteger, Label: "Campaigns Sharing This Budget"},
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

	amount, err := gads.MoneyToMicros("daily_budget", inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	if amount == nil || *amount <= 0 {
		return gads.ErrorResult("a daily budget greater than zero is required"), nil
	}

	client := gads.NewClient(token, login)

	budgetID := gads.OptionalString("budget_id", inputs)
	shared := 0
	if budgetID == "" {
		// A campaign does not hold its budget amount; it references a separate
		// CampaignBudget resource. Taking a campaign id and resolving the budget
		// here saves the caller a lookup they would otherwise always have to do.
		campaignID, err := gads.RequiredString("campaign_id", inputs)
		if err != nil {
			return gads.ErrorResult("give either a campaign ID or a budget ID"), nil
		}
		rows, err := client.Search(flow, customerID, gads.BuildQuery(
			[]string{"campaign_budget.id, campaign_budget.explicitly_shared, campaign_budget.reference_count"},
			"campaign", []string{"campaign.id = " + gads.QuoteGAQL(campaignID)}, "", nil))
		if err != nil {
			return gads.ErrorResult(err.Error()), nil
		}
		if len(rows) == 0 {
			return gads.ErrorResult(fmt.Sprintf("campaign %s was not found in account %s", campaignID, customerID)), nil
		}
		flat := gads.FlattenRow(rows[0])
		budgetID, _ = flat["campaign_budget.id"].(string)
		if budgetID == "" {
			return gads.ErrorResult(fmt.Sprintf("campaign %s has no budget to change", campaignID)), nil
		}
		if count, ok := flat["campaign_budget.reference_count"].(string); ok {
			if n, err := strconv.Atoi(count); err == nil {
				shared = n
			}
		}
	}

	operations := []interface{}{map[string]interface{}{
		"update": map[string]interface{}{
			"resourceName": fmt.Sprintf("customers/%s/campaignBudgets/%s", customerID, budgetID),
			"amountMicros": strconv.FormatInt(*amount, 10),
		},
		"updateMask": "amount_micros",
	}}
	opt := gads.MutateOptions{ValidateOnly: gads.OptionalBool("validate_only", inputs), PartialFailure: false}

	resp, err := client.Mutate(flow, customerID, "campaignBudgets", operations, opt)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Set budget %s to %s per day", budgetID, gads.MicrosToMajor(*amount))
	// A shared budget funds several campaigns at once, so changing it moves
	// money for all of them. Silently doing that to a campaign the caller did
	// not name is the kind of surprise worth a sentence.
	if shared > 1 {
		summary += fmt.Sprintf(" — note this budget is shared by %d campaigns, all of which are affected", shared)
	}
	return gads.MutateResult(resp, opt, summary, map[string]interface{}{"shared_with_campaigns": shared}), nil
}

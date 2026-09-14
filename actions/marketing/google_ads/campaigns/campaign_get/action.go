// Package campaign_get reads one Google Ads campaign.
package campaign_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Campaign: Get"
	Description  = "Read one Google Ads campaign's settings, budget and bidding strategy by ID."
	Summary      = "Read a Google Ads campaign"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+magnifying-glass"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

const campaignFields = "campaign.id, campaign.name, campaign.status, campaign.advertising_channel_type, " +
	"campaign.advertising_channel_sub_type, campaign.bidding_strategy_type, campaign.start_date_time, " +
	"campaign.end_date_time, campaign.tracking_url_template, campaign.final_url_suffix, " +
	"campaign.network_settings.target_google_search, campaign.network_settings.target_search_network, " +
	"campaign.network_settings.target_content_network, campaign.optimization_score, " +
	"campaign_budget.id, campaign_budget.name, campaign_budget.amount_micros, campaign_budget.period, " +
	"campaign_budget.delivery_method, campaign_budget.explicitly_shared"

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Campaign"},
	{Name: "table", Type: core.ConnectionTypeObject, Label: "Campaign (flattened)"},
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

	query := gads.BuildQuery([]string{campaignFields}, "campaign",
		[]string{"campaign.id = " + gads.QuoteGAQL(campaignID)}, "", nil)

	rows, err := gads.NewClient(token, login).Search(flow, customerID, query)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	if len(rows) == 0 {
		return gads.ErrorResult(fmt.Sprintf("campaign %s was not found in account %s", campaignID, customerID)), nil
	}
	return gads.RowsResult(rows, "Read campaign "+campaignID, nil), nil
}

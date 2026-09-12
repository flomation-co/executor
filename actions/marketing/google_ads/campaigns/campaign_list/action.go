// Package campaign_list lists campaigns in a Google Ads account.
package campaign_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Campaigns: List"
	Description  = "List Google Ads campaigns with status, budget, bidding and optional performance metrics."
	Summary      = "List campaigns in Google Ads"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+list"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

const campaignFields = "campaign.id, campaign.name, campaign.status, campaign.advertising_channel_type, " +
	"campaign.advertising_channel_sub_type, campaign.bidding_strategy_type, campaign.start_date_time, " +
	"campaign.end_date_time, campaign_budget.id, campaign_budget.amount_micros, campaign_budget.period"

// Metrics are opt-in, not default. Adding them forces Google to aggregate over
// a date range, which is slower, costs more of the daily operation quota, and
// silently DROPS campaigns with no activity in the window — so a plain "list my
// campaigns" would quietly omit the paused ones the caller was looking for.
const metricFields = "metrics.impressions, metrics.clicks, metrics.cost_micros, metrics.conversions, " +
	"metrics.conversions_value, metrics.average_cpc, metrics.ctr"

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Filter by Status", Options: []core.ConnectionOption{{Name: "Any (excluding removed)", Value: ""}, {Name: "Enabled", Value: "ENABLED"}, {Name: "Paused", Value: "PAUSED"}, {Name: "Removed", Value: "REMOVED"}}},
	{Name: "name_contains", Type: core.ConnectionTypeString, Label: "Filter by Name contains"},
	{Name: "include_metrics", Type: core.ConnectionTypeBoolean, Label: "Include performance metrics (needs a date range)"},
	{Name: "date_range", Type: core.ConnectionTypeString, Label: "Date Range (for metrics)", Options: []core.ConnectionOption{{Name: "Last 7 days", Value: "LAST_7_DAYS"}, {Name: "Last 14 days", Value: "LAST_14_DAYS"}, {Name: "Last 30 days", Value: "LAST_30_DAYS"}, {Name: "This month", Value: "THIS_MONTH"}, {Name: "Last month", Value: "LAST_MONTH"}, {Name: "Today", Value: "TODAY"}, {Name: "Yesterday", Value: "YESTERDAY"}, {Name: "Custom", Value: "CUSTOM"}}},
	{Name: "date_from", Type: core.ConnectionTypeString, Label: "Start Date (custom range, YYYY-MM-DD)"},
	{Name: "date_to", Type: core.ConnectionTypeString, Label: "End Date (custom range, YYYY-MM-DD)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum campaigns to return", Placeholder: "200"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Campaigns"},
	{Name: "table", Type: core.ConnectionTypeObject, Label: "Campaigns (flattened)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "query", Type: core.ConnectionTypeString, Label: "GAQL Query Used"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, customerID, login, err := gads.GetAuth(inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	fields := []string{campaignFields}
	var conditions []string

	if status := gads.OptionalString("status", inputs); status != "" {
		conditions = append(conditions, "campaign.status = "+gads.QuoteGAQL(status))
	} else {
		// REMOVED campaigns are permanent tombstones and accumulate forever, so
		// excluding them by default keeps the list to what can still be acted on.
		conditions = append(conditions, "campaign.status != 'REMOVED'")
	}
	if name := gads.OptionalString("name_contains", inputs); name != "" {
		conditions = append(conditions, "campaign.name LIKE "+gads.QuoteGAQL("%"+name+"%"))
	}

	if gads.OptionalBool("include_metrics", inputs) {
		fields = append(fields, metricFields)
		preset := gads.OptionalString("date_range", inputs)
		if preset == "" {
			preset = "LAST_30_DAYS"
		}
		clause, err := gads.DateRangeClause(preset, gads.OptionalString("date_from", inputs), gads.OptionalString("date_to", inputs))
		if err != nil {
			return gads.ErrorResult(err.Error()), nil
		}
		conditions = append(conditions, clause)
	}

	limit := gads.OptionalInt("limit", inputs)
	if limit == nil {
		fallback := int64(200)
		limit = &fallback
	}

	query := gads.BuildQuery(fields, "campaign", conditions, "campaign.name", limit)
	rows, err := gads.NewClient(token, login).Search(flow, customerID, query)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	return gads.RowsResult(rows, fmt.Sprintf("Found %d campaign(s)", len(rows)), map[string]interface{}{"query": query}), nil
}

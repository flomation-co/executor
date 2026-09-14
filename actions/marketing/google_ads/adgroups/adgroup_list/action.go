// Package adgroup_list lists ad groups in a Google Ads account or campaign.
package adgroup_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Ad Groups: List"
	Description  = "List Google Ads ad groups, optionally within one campaign, with optional performance metrics."
	Summary      = "List ad groups in Google Ads"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+layer-group"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

const adGroupFields = "ad_group.id, ad_group.name, ad_group.status, ad_group.type, ad_group.cpc_bid_micros, " +
	"ad_group.target_cpa_micros, campaign.id, campaign.name"

const metricFields = "metrics.impressions, metrics.clicks, metrics.cost_micros, metrics.conversions, metrics.ctr"

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID (leave blank for the whole account)"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Filter by Status", Options: []core.ConnectionOption{{Name: "Any (excluding removed)", Value: ""}, {Name: "Enabled", Value: "ENABLED"}, {Name: "Paused", Value: "PAUSED"}, {Name: "Removed", Value: "REMOVED"}}},
	{Name: "name_contains", Type: core.ConnectionTypeString, Label: "Filter by Name contains"},
	{Name: "include_metrics", Type: core.ConnectionTypeBoolean, Label: "Include performance metrics (needs a date range)"},
	{Name: "date_range", Type: core.ConnectionTypeString, Label: "Date Range (for metrics)", Options: []core.ConnectionOption{{Name: "Last 7 days", Value: "LAST_7_DAYS"}, {Name: "Last 14 days", Value: "LAST_14_DAYS"}, {Name: "Last 30 days", Value: "LAST_30_DAYS"}, {Name: "This month", Value: "THIS_MONTH"}, {Name: "Last month", Value: "LAST_MONTH"}, {Name: "Custom", Value: "CUSTOM"}}},
	{Name: "date_from", Type: core.ConnectionTypeString, Label: "Start Date (custom range, YYYY-MM-DD)"},
	{Name: "date_to", Type: core.ConnectionTypeString, Label: "End Date (custom range, YYYY-MM-DD)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum ad groups to return", Placeholder: "200"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Ad Groups"},
	{Name: "table", Type: core.ConnectionTypeObject, Label: "Ad Groups (flattened)"},
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

	fields := []string{adGroupFields}
	var conditions []string

	if id := gads.OptionalString("campaign_id", inputs); id != "" {
		conditions = append(conditions, "campaign.id = "+gads.QuoteGAQL(id))
	}
	if status := gads.OptionalString("status", inputs); status != "" {
		conditions = append(conditions, "ad_group.status = "+gads.QuoteGAQL(status))
	} else {
		conditions = append(conditions, "ad_group.status != 'REMOVED'")
	}
	if name := gads.OptionalString("name_contains", inputs); name != "" {
		conditions = append(conditions, "ad_group.name LIKE "+gads.QuoteGAQL("%"+name+"%"))
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

	query := gads.BuildQuery(fields, "ad_group", conditions, "campaign.name, ad_group.name", limit)
	rows, err := gads.NewClient(token, login).Search(flow, customerID, query)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	return gads.RowsResult(rows, fmt.Sprintf("Found %d ad group(s)", len(rows)), map[string]interface{}{"query": query}), nil
}

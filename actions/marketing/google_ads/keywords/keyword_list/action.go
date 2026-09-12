// Package keyword_list lists keywords with their performance.
package keyword_list

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Keywords: List"
	Description  = "List Google Ads keywords with match type, bid, quality score and performance."
	Summary      = "List keywords in Google Ads"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+key"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

// keyword_view rather than ad_group_criterion: the view is the reporting
// surface, so it is the one that carries metrics, and it already excludes the
// non-keyword criteria (audiences, placements) that share the criterion table.
const keywordFields = "ad_group_criterion.criterion_id, ad_group_criterion.keyword.text, " +
	"ad_group_criterion.keyword.match_type, ad_group_criterion.status, " +
	"ad_group_criterion.cpc_bid_micros, ad_group_criterion.effective_cpc_bid_micros, " +
	"ad_group_criterion.quality_info.quality_score, ad_group.id, ad_group.name, campaign.id, campaign.name"

const metricFields = "metrics.impressions, metrics.clicks, metrics.cost_micros, metrics.conversions, " +
	"metrics.conversions_value, metrics.average_cpc, metrics.ctr"

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "ad_group_id", Type: core.ConnectionTypeString, Label: "Ad Group ID (leave blank for the whole account)"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID (leave blank for the whole account)"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Filter by Status", Options: []core.ConnectionOption{{Name: "Any (excluding removed)", Value: ""}, {Name: "Enabled", Value: "ENABLED"}, {Name: "Paused", Value: "PAUSED"}, {Name: "Removed", Value: "REMOVED"}}},
	{Name: "text_contains", Type: core.ConnectionTypeString, Label: "Filter by Keyword text contains"},
	{Name: "include_metrics", Type: core.ConnectionTypeBoolean, Label: "Include performance metrics (needs a date range)"},
	{Name: "date_range", Type: core.ConnectionTypeString, Label: "Date Range (for metrics)", Options: []core.ConnectionOption{{Name: "Last 7 days", Value: "LAST_7_DAYS"}, {Name: "Last 14 days", Value: "LAST_14_DAYS"}, {Name: "Last 30 days", Value: "LAST_30_DAYS"}, {Name: "This month", Value: "THIS_MONTH"}, {Name: "Last month", Value: "LAST_MONTH"}, {Name: "Custom", Value: "CUSTOM"}}},
	{Name: "date_from", Type: core.ConnectionTypeString, Label: "Start Date (custom range, YYYY-MM-DD)"},
	{Name: "date_to", Type: core.ConnectionTypeString, Label: "End Date (custom range, YYYY-MM-DD)"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort by", Options: []core.ConnectionOption{{Name: "Keyword text", Value: "ad_group_criterion.keyword.text"}, {Name: "Cost (highest first)", Value: "metrics.cost_micros DESC"}, {Name: "Clicks (highest first)", Value: "metrics.clicks DESC"}, {Name: "Conversions (highest first)", Value: "metrics.conversions DESC"}}},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum keywords to return", Placeholder: "500"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Keywords"},
	{Name: "table", Type: core.ConnectionTypeObject, Label: "Keywords (flattened)"},
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

	fields := []string{keywordFields}
	var conditions []string

	if id := gads.OptionalString("ad_group_id", inputs); id != "" {
		conditions = append(conditions, "ad_group.id = "+gads.QuoteGAQL(id))
	}
	if id := gads.OptionalString("campaign_id", inputs); id != "" {
		conditions = append(conditions, "campaign.id = "+gads.QuoteGAQL(id))
	}
	if status := gads.OptionalString("status", inputs); status != "" {
		conditions = append(conditions, "ad_group_criterion.status = "+gads.QuoteGAQL(status))
	} else {
		conditions = append(conditions, "ad_group_criterion.status != 'REMOVED'")
	}
	if text := gads.OptionalString("text_contains", inputs); text != "" {
		conditions = append(conditions, "ad_group_criterion.keyword.text LIKE "+gads.QuoteGAQL("%"+text+"%"))
	}

	orderBy := gads.OptionalString("order_by", inputs)
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
	} else if strings.HasPrefix(orderBy, "metrics.") {
		// Sorting by a metric that was never selected is a query error rather
		// than an empty result, so catch it here where the fix is obvious.
		return gads.ErrorResult("sorting by a metric needs \"Include performance metrics\" switched on"), nil
	}
	if orderBy == "" {
		orderBy = "ad_group_criterion.keyword.text"
	}

	limit := gads.OptionalInt("limit", inputs)
	if limit == nil {
		fallback := int64(500)
		limit = &fallback
	}

	query := gads.BuildQuery(fields, "keyword_view", conditions, orderBy, limit)
	rows, err := gads.NewClient(token, login).Search(flow, customerID, query)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	return gads.RowsResult(rows, fmt.Sprintf("Found %d keyword(s)", len(rows)), map[string]interface{}{"query": query}), nil
}

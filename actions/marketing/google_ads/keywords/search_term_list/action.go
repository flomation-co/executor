// Package search_term_list reports the search terms that actually triggered
// ads.
package search_term_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Search Terms: List"
	Description  = "List the real search terms that triggered your ads, with cost and conversions."
	Summary      = "See what people actually searched for"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+magnifying-glass"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

// The search terms report is the heart of Google Ads optimisation: it is the
// only place that shows what people ACTUALLY typed, as opposed to the keywords
// that were bid on. Reading it, finding the terms that cost money without
// converting, and adding those as negatives is the loop this integration exists
// to automate — this action plus Negative Keyword: Add is that whole loop.
const searchTermFields = "search_term_view.search_term, search_term_view.status, " +
	"segments.keyword.info.text, segments.keyword.info.match_type, " +
	"ad_group.id, ad_group.name, campaign.id, campaign.name, " +
	"metrics.impressions, metrics.clicks, metrics.cost_micros, metrics.conversions, " +
	"metrics.conversions_value, metrics.ctr"

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID (leave blank for the whole account)"},
	{Name: "ad_group_id", Type: core.ConnectionTypeString, Label: "Ad Group ID (leave blank for the whole account)"},
	{Name: "date_range", Type: core.ConnectionTypeString, Label: "Date Range", Options: []core.ConnectionOption{{Name: "Last 7 days", Value: "LAST_7_DAYS"}, {Name: "Last 14 days", Value: "LAST_14_DAYS"}, {Name: "Last 30 days", Value: "LAST_30_DAYS"}, {Name: "This month", Value: "THIS_MONTH"}, {Name: "Last month", Value: "LAST_MONTH"}, {Name: "Custom", Value: "CUSTOM"}}},
	{Name: "date_from", Type: core.ConnectionTypeString, Label: "Start Date (custom range, YYYY-MM-DD)"},
	{Name: "date_to", Type: core.ConnectionTypeString, Label: "End Date (custom range, YYYY-MM-DD)"},
	{Name: "min_clicks", Type: core.ConnectionTypeInteger, Label: "Only terms with at least this many clicks"},
	{Name: "zero_conversions_only", Type: core.ConnectionTypeBoolean, Label: "Only terms that cost money and converted nothing"},
	{Name: "exclude_already_negative", Type: core.ConnectionTypeBoolean, Label: "Hide terms already excluded as negatives"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum search terms to return", Placeholder: "500"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Search Terms"},
	{Name: "table", Type: core.ConnectionTypeObject, Label: "Search Terms (flattened)"},
	{Name: "search_terms", Type: core.ConnectionTypeObject, Label: "Search Terms (text only, ready to add as negatives)"},
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

	preset := gads.OptionalString("date_range", inputs)
	if preset == "" {
		preset = "LAST_30_DAYS"
	}
	dateClause, err := gads.DateRangeClause(preset, gads.OptionalString("date_from", inputs), gads.OptionalString("date_to", inputs))
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	conditions := []string{dateClause}
	if id := gads.OptionalString("campaign_id", inputs); id != "" {
		conditions = append(conditions, "campaign.id = "+gads.QuoteGAQL(id))
	}
	if id := gads.OptionalString("ad_group_id", inputs); id != "" {
		conditions = append(conditions, "ad_group.id = "+gads.QuoteGAQL(id))
	}
	if clicks := gads.OptionalInt("min_clicks", inputs); clicks != nil && *clicks > 0 {
		conditions = append(conditions, fmt.Sprintf("metrics.clicks >= %d", *clicks))
	}
	if gads.OptionalBool("zero_conversions_only", inputs) {
		// Cost, not clicks: a term with clicks but no cost is a free
		// impression and not the waste anyone is hunting for.
		conditions = append(conditions, "metrics.conversions = 0", "metrics.cost_micros > 0")
	}
	if gads.OptionalBool("exclude_already_negative", inputs) {
		// EXCLUDED means a negative keyword already blocks this term, so
		// re-adding it would be a no-op that still costs an operation.
		conditions = append(conditions, "search_term_view.status != 'EXCLUDED'")
	}

	limit := gads.OptionalInt("limit", inputs)
	if limit == nil {
		fallback := int64(500)
		limit = &fallback
	}

	query := gads.BuildQuery([]string{searchTermFields}, "search_term_view", conditions, "metrics.cost_micros DESC", limit)
	rows, err := gads.NewClient(token, login).Search(flow, customerID, query)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	// A bare list of the term strings, so the natural next node — Negative
	// Keyword: Add, which takes one term per line — can be wired straight to
	// this one without a script node in between.
	terms := make([]string, 0, len(rows))
	for _, row := range gads.FlattenRows(rows) {
		if term, ok := row["search_term_view.search_term"].(string); ok && term != "" {
			terms = append(terms, term)
		}
	}

	return gads.RowsResult(rows, fmt.Sprintf("Found %d search term(s), highest cost first", len(rows)), map[string]interface{}{
		"query":        query,
		"search_terms": terms,
	}), nil
}

// Package report_performance runs a curated Google Ads performance report.
package report_performance

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Report: Performance"
	Description  = "Report Google Ads performance at account, campaign, ad group, ad or keyword level."
	Summary      = "Report on Google Ads performance"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+chart-line"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

// level picks both the GAQL resource and the identifying columns that go with
// it. Keeping the pair together is the point of this action: choosing "keyword"
// and then having to remember that the resource is keyword_view, not keyword, is
// exactly the knowledge a curated action should absorb.
type level struct {
	resource string
	columns  string
	orderBy  string
}

var levels = map[string]level{
	"ACCOUNT": {
		resource: "customer",
		columns:  "customer.id, customer.descriptive_name",
		orderBy:  "customer.id",
	},
	"CAMPAIGN": {
		resource: "campaign",
		columns:  "campaign.id, campaign.name, campaign.status, campaign.advertising_channel_type",
		orderBy:  "campaign.name",
	},
	"AD_GROUP": {
		resource: "ad_group",
		columns:  "campaign.id, campaign.name, ad_group.id, ad_group.name, ad_group.status",
		orderBy:  "campaign.name, ad_group.name",
	},
	"AD": {
		resource: "ad_group_ad",
		columns:  "campaign.name, ad_group.name, ad_group_ad.ad.id, ad_group_ad.ad.type, ad_group_ad.status",
		orderBy:  "campaign.name, ad_group.name",
	},
	"KEYWORD": {
		resource: "keyword_view",
		columns:  "campaign.name, ad_group.name, ad_group_criterion.criterion_id, ad_group_criterion.keyword.text, ad_group_criterion.keyword.match_type",
		orderBy:  "campaign.name, ad_group.name",
	},
	"DEVICE": {
		resource: "campaign",
		columns:  "campaign.id, campaign.name",
		orderBy:  "campaign.name",
	},
	"GEO": {
		resource: "geographic_view",
		columns:  "campaign.name, geographic_view.country_criterion_id, geographic_view.location_type",
		orderBy:  "campaign.name",
	},
}

// The default metric set: what someone means by "performance" before they ask
// for anything more specific.
const defaultMetrics = "metrics.impressions, metrics.clicks, metrics.cost_micros, metrics.conversions, " +
	"metrics.conversions_value, metrics.ctr, metrics.average_cpc"

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "level", Type: core.ConnectionTypeString, Label: "Report On", Required: true, Options: []core.ConnectionOption{{Name: "Whole account", Value: "ACCOUNT"}, {Name: "Campaigns", Value: "CAMPAIGN"}, {Name: "Ad groups", Value: "AD_GROUP"}, {Name: "Ads", Value: "AD"}, {Name: "Keywords", Value: "KEYWORD"}, {Name: "Devices", Value: "DEVICE"}, {Name: "Locations", Value: "GEO"}}},
	{Name: "date_range", Type: core.ConnectionTypeString, Label: "Date Range", Required: true, Options: []core.ConnectionOption{{Name: "Today", Value: "TODAY"}, {Name: "Yesterday", Value: "YESTERDAY"}, {Name: "Last 7 days", Value: "LAST_7_DAYS"}, {Name: "Last 14 days", Value: "LAST_14_DAYS"}, {Name: "Last 30 days", Value: "LAST_30_DAYS"}, {Name: "This week", Value: "THIS_WEEK_MON_TODAY"}, {Name: "Last week", Value: "LAST_WEEK_MON_SUN"}, {Name: "This month", Value: "THIS_MONTH"}, {Name: "Last month", Value: "LAST_MONTH"}, {Name: "This quarter", Value: "THIS_QUARTER"}, {Name: "Last quarter", Value: "LAST_QUARTER"}, {Name: "This year", Value: "THIS_YEAR"}, {Name: "Last year", Value: "LAST_YEAR"}, {Name: "Custom", Value: "CUSTOM"}}},
	{Name: "date_from", Type: core.ConnectionTypeString, Label: "Start Date (custom range, YYYY-MM-DD)"},
	{Name: "date_to", Type: core.ConnectionTypeString, Label: "End Date (custom range, YYYY-MM-DD)"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Limit to one Campaign ID"},
	{Name: "ad_group_id", Type: core.ConnectionTypeString, Label: "Limit to one Ad Group ID"},
	{Name: "break_down_by", Type: core.ConnectionTypeMultiSelect, Label: "Break the numbers down by", Options: []core.ConnectionOption{{Name: "Day", Value: "segments.date"}, {Name: "Week", Value: "segments.week"}, {Name: "Month", Value: "segments.month"}, {Name: "Device", Value: "segments.device"}, {Name: "Network", Value: "segments.ad_network_type"}, {Name: "Conversion action", Value: "segments.conversion_action_name"}}},
	{Name: "metrics", Type: core.ConnectionTypeMultiSelect, Label: "Metrics (leave blank for the usual set)", Options: []core.ConnectionOption{{Name: "Impressions", Value: "metrics.impressions"}, {Name: "Clicks", Value: "metrics.clicks"}, {Name: "Cost", Value: "metrics.cost_micros"}, {Name: "Conversions", Value: "metrics.conversions"}, {Name: "Conversion value", Value: "metrics.conversions_value"}, {Name: "Click-through rate", Value: "metrics.ctr"}, {Name: "Average CPC", Value: "metrics.average_cpc"}, {Name: "Cost per conversion", Value: "metrics.cost_per_conversion"}, {Name: "Conversion rate", Value: "metrics.conversions_from_interactions_rate"}, {Name: "Search impression share", Value: "metrics.search_impression_share"}, {Name: "Absolute top impression share", Value: "metrics.absolute_top_impression_percentage"}, {Name: "Budget lost impression share", Value: "metrics.search_budget_lost_impression_share"}, {Name: "All conversions", Value: "metrics.all_conversions"}}},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort by", Options: []core.ConnectionOption{{Name: "Default for this level", Value: ""}, {Name: "Cost (highest first)", Value: "metrics.cost_micros DESC"}, {Name: "Clicks (highest first)", Value: "metrics.clicks DESC"}, {Name: "Impressions (highest first)", Value: "metrics.impressions DESC"}, {Name: "Conversions (highest first)", Value: "metrics.conversions DESC"}}},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum rows to return", Placeholder: "500"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Report Rows"},
	{Name: "table", Type: core.ConnectionTypeObject, Label: "Report Rows (flattened, money converted)"},
	{Name: "totals", Type: core.ConnectionTypeObject, Label: "Totals"},
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

	name := strings.ToUpper(gads.OptionalString("level", inputs))
	if name == "" {
		name = "CAMPAIGN"
	}
	chosen, ok := levels[name]
	if !ok {
		return gads.ErrorResult(fmt.Sprintf("%q is not a reporting level — choose account, campaign, ad group, ad, keyword, device or location", name)), nil
	}

	clause, err := gads.DateRangeClause(
		gads.OptionalString("date_range", inputs),
		gads.OptionalString("date_from", inputs),
		gads.OptionalString("date_to", inputs))
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	if clause == "" {
		return gads.ErrorResult("a date range is required for a performance report"), nil
	}
	conditions := []string{clause}

	if id := gads.OptionalString("campaign_id", inputs); id != "" {
		conditions = append(conditions, "campaign.id = "+gads.QuoteGAQL(id))
	}
	if id := gads.OptionalString("ad_group_id", inputs); id != "" {
		conditions = append(conditions, "ad_group.id = "+gads.QuoteGAQL(id))
	}

	fields := []string{chosen.columns}

	segments := gads.OptionalList("break_down_by", inputs)
	// A device report is a campaign report segmented by device — there is no
	// separate resource for it — so the level quietly supplies the segment.
	if name == "DEVICE" && !contains(segments, "segments.device") {
		segments = append(segments, "segments.device")
	}
	if len(segments) > 0 {
		if err := validateSegments(segments); err != nil {
			return gads.ErrorResult(err.Error()), nil
		}
		fields = append(fields, strings.Join(segments, ", "))
	}

	metrics := gads.OptionalList("metrics", inputs)
	if len(metrics) == 0 {
		fields = append(fields, defaultMetrics)
		metrics = strings.Split(strings.ReplaceAll(defaultMetrics, " ", ""), ",")
	} else {
		if err := validateMetrics(metrics); err != nil {
			return gads.ErrorResult(err.Error()), nil
		}
		fields = append(fields, strings.Join(metrics, ", "))
	}

	orderBy := gads.OptionalString("order_by", inputs)
	if orderBy == "" {
		orderBy = chosen.orderBy
	}

	limit := gads.OptionalInt("limit", inputs)
	if limit == nil {
		fallback := int64(500)
		limit = &fallback
	}

	query := gads.BuildQuery(fields, chosen.resource, conditions, orderBy, limit)
	rows, err := gads.NewClient(token, login).Search(flow, customerID, query)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	table := gads.FlattenRows(rows)
	totals := sumMetrics(table, metrics)

	summary := fmt.Sprintf("%s report, %d row(s)", strings.ToLower(strings.ReplaceAll(name, "_", " ")), len(rows))
	if headline := describeTotals(totals); headline != "" {
		summary += " — " + headline
	}

	return gads.RowsResult(rows, summary, map[string]interface{}{
		"query":  query,
		"totals": totals,
	}), nil
}

// validateMetrics and validateSegments catch a field that has been put in the
// wrong box. Both are free-text-capable multi-selects, so a ${...} value can
// deliver anything; a segment listed as a metric produces a GAQL error naming
// only the query, whereas this names the field and says where it belongs.
func validateMetrics(values []string) error {
	for _, v := range values {
		if !strings.HasPrefix(v, "metrics.") {
			return fmt.Errorf("%q is not a metric — metrics start with \"metrics.\"; if it is a breakdown like segments.date, put it in \"Break the numbers down by\" instead", v)
		}
	}
	return nil
}

func validateSegments(values []string) error {
	for _, v := range values {
		if !strings.HasPrefix(v, "segments.") {
			return fmt.Errorf("%q is not a breakdown — breakdowns start with \"segments.\"", v)
		}
	}
	return nil
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// sumMetrics totals the numeric columns across every row.
//
// Google returns per-row figures and no total, so anything summarising a report
// — an alert on spend, a weekly digest — would otherwise have to add them up in
// a script node. Money is summed from the raw micros and rendered once at the
// end, which avoids accumulating rounding across hundreds of rows.
func sumMetrics(table []map[string]interface{}, metrics []string) map[string]interface{} {
	totals := map[string]interface{}{}
	for _, metric := range metrics {
		metric = strings.TrimSpace(metric)
		if metric == "" {
			continue
		}
		// Rates and shares do not add up — a total CTR of 480% is worse than no
		// answer at all, and the correct figure has to be recomputed from its
		// parts rather than summed.
		if isRate(metric) {
			continue
		}

		var sum float64
		var found bool
		for _, row := range table {
			if value, ok := numeric(row[metric]); ok {
				sum += value
				found = true
			}
		}
		if !found {
			continue
		}

		totals[metric] = sum
		if strings.HasSuffix(metric, "_micros") {
			totals[strings.TrimSuffix(metric, "_micros")] = gads.MicrosToMajor(int64(sum))
		}
	}
	return totals
}

func isRate(metric string) bool {
	for _, suffix := range []string{"ctr", "_rate", "_share", "_percentage", "average_cpc", "cost_per_conversion"} {
		if strings.HasSuffix(metric, suffix) {
			return true
		}
	}
	return false
}

func describeTotals(totals map[string]interface{}) string {
	var parts []string
	if v, ok := totals["metrics.impressions"].(float64); ok {
		parts = append(parts, fmt.Sprintf("%.0f impressions", v))
	}
	if v, ok := totals["metrics.clicks"].(float64); ok {
		parts = append(parts, fmt.Sprintf("%.0f clicks", v))
	}
	if v, ok := totals["metrics.cost"].(string); ok {
		parts = append(parts, v+" spent")
	}
	if v, ok := totals["metrics.conversions"].(float64); ok {
		parts = append(parts, fmt.Sprintf("%.1f conversions", v))
	}
	return strings.Join(parts, ", ")
}

func numeric(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		var f float64
		if _, err := fmt.Sscanf(n, "%g", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

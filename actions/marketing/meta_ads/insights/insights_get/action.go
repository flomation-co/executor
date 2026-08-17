// Package insights_get pulls Meta advertising performance data at any level of
// the ad hierarchy.
package insights_get

import (
	"fmt"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Insights: Get"
	Description  = "Get Meta ad performance (spend, impressions, clicks, conversions) for an account, campaign, ad set or ad."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+chart-line"
	Date         = "17/08/2026"
	Type         = core.ActionTypeAction
)

const defaultFields = "date_start,date_stop,spend,impressions,clicks,ctr,cpc,cpm,reach,frequency,actions,cost_per_action_type"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	// One id input rather than four: the /insights edge hangs off every object
	// in the hierarchy and behaves identically, so splitting this into separate
	// actions per level would be four copies of the same code.
	{Name: "object_id", Type: core.ConnectionTypeString, Label: "Ad Account / Campaign / Ad Set / Ad ID", Placeholder: "act_1234567890, or a campaign/ad set/ad ID", Required: true},
	// `level` controls the granularity of the ROWS, independent of which object
	// you asked about: querying an account at level=campaign gives one row per
	// campaign. Leaving it unset returns a single aggregate row, which is the
	// most common cause of "why is there only one result".
	{Name: "level", Type: core.ConnectionTypeString, Label: "Breakdown Level", Options: []core.ConnectionOption{
		{Name: "Aggregate (default — one row)", Value: ""},
		{Name: "Per account", Value: "account"},
		{Name: "Per campaign", Value: "campaign"},
		{Name: "Per ad set", Value: "adset"},
		{Name: "Per ad", Value: "ad"},
	}},
	{Name: "date_preset", Type: core.ConnectionTypeString, Label: "Date Range", Options: []core.ConnectionOption{
		{Name: "Today", Value: "today"},
		{Name: "Yesterday", Value: "yesterday"},
		{Name: "Last 7 days", Value: "last_7d"},
		{Name: "Last 14 days", Value: "last_14d"},
		{Name: "Last 30 days", Value: "last_30d"},
		{Name: "This month", Value: "this_month"},
		{Name: "Last month", Value: "last_month"},
		{Name: "Maximum", Value: "maximum"},
	}},
	{Name: "time_range_since", Type: core.ConnectionTypeString, Label: "Custom Range — Since (YYYY-MM-DD)", Placeholder: "2026-08-01"},
	{Name: "time_range_until", Type: core.ConnectionTypeString, Label: "Custom Range — Until (YYYY-MM-DD)", Placeholder: "2026-08-17"},
	{Name: "time_increment", Type: core.ConnectionTypeString, Label: "Time Increment", Options: []core.ConnectionOption{
		{Name: "Whole range as one row", Value: ""},
		{Name: "Daily", Value: "1"},
		{Name: "Weekly", Value: "7"},
		{Name: "Monthly", Value: "monthly"},
	}},
	{Name: "breakdowns", Type: core.ConnectionTypeString, Label: "Breakdowns", Placeholder: "age,gender  ·  country  ·  publisher_platform (comma-separated)"},
	{Name: "filtering", Type: core.ConnectionTypeText, Label: "Filtering (JSON array)", Placeholder: `[{"field":"spend","operator":"GREATER_THAN","value":100}]`},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Metrics", Placeholder: defaultFields},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "100"},
	{Name: "after", Type: core.ConnectionTypeString, Label: "Page Cursor (from a previous run)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Insight Rows"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Row Count"},
	{Name: "next_cursor", Type: core.ConnectionTypeString, Label: "Next Page Cursor"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, secret, err := meta.GetAuth(inputs)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}
	objectID, err := meta.RequiredString("object_id", inputs)
	if err != nil {
		return meta.ErrorResult("an ad account, campaign, ad set or ad ID is required"), nil
	}

	p := url.Values{"fields": {meta.Fields("fields", inputs, defaultFields)}}
	meta.SetParam(p, "level", "level", inputs)
	meta.SetParam(p, "time_increment", "time_increment", inputs)
	meta.SetParam(p, "breakdowns", "breakdowns", inputs)
	meta.SetParam(p, "limit", "limit", inputs)
	meta.SetParam(p, "after", "after", inputs)

	// A custom range and a preset are mutually exclusive at Meta; sending both
	// makes it silently pick one, so resolve it here and say which was used.
	since := meta.OptionalString("time_range_since", inputs)
	until := meta.OptionalString("time_range_until", inputs)
	rangeUsed := ""
	switch {
	case since != "" && until != "":
		p.Set("time_range", fmt.Sprintf(`{"since":"%s","until":"%s"}`, since, until))
		rangeUsed = since + " to " + until
	case since != "" || until != "":
		return meta.ErrorResult("a custom date range needs BOTH Since and Until — set the pair, or use the Date Range preset instead"), nil
	default:
		preset := meta.OptionalString("date_preset", inputs)
		if preset == "" {
			preset = "last_30d"
		}
		p.Set("date_preset", preset)
		rangeUsed = preset
	}

	if f := meta.OptionalString("filtering", inputs); f != "" {
		if !strings.HasPrefix(strings.TrimSpace(f), "[") {
			return meta.ErrorResult("Filtering must be a JSON ARRAY of filter objects, e.g. [{\"field\":\"spend\",\"operator\":\"GREATER_THAN\",\"value\":100}]"), nil
		}
		if err := meta.SetJSONParam(p, "filtering", "filtering", inputs); err != nil {
			return meta.ErrorResult(err.Error()), nil
		}
	}

	// The account edge needs the act_ prefix; every other object id is used
	// bare. Normalising only when it already looks like an account id avoids
	// mangling a campaign id into act_<campaign>.
	path := "/" + strings.TrimSpace(objectID)
	if strings.HasPrefix(objectID, "act_") {
		path = meta.AccountPath(objectID)
	}

	resp, err := meta.NewClient(token, secret).Get(flow, path+"/insights", p)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	rows := meta.Data(resp)
	next := meta.NextCursor(resp)

	summary := fmt.Sprintf("Retrieved %d insight row(s) for %s over %s", len(rows), objectID, rangeUsed)
	if len(rows) == 0 {
		// An empty result is ambiguous — no delivery, or a range with no data —
		// and silently returning nothing invites the reader to assume a broken
		// filter.
		summary += ". No data: the object may not have delivered in this period, or the range may predate it."
	}
	if next != "" {
		summary += " (more pages available — pass next_cursor back in as the Page Cursor)"
	}
	return meta.ListResult(rows, summary, map[string]interface{}{"next_cursor": next}), nil
}

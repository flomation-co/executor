package marketing_sendgrid_stats_get

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Get Stats"
	Description  = "Retrieve your SendGrid email statistics — requests, delivered, opens, clicks, bounces, and more — for a date range, optionally aggregated by day, week, or month. Dates use YYYY-MM-DD format."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+pie-chart"
	Date         = "09/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "SendGrid API key (SendGrid → Settings → API Keys), e.g. ${secrets.sendgrid_api}", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "Global", Value: ""},
			{Name: "EU (data residency)", Value: "eu"},
		},
		Placeholder: "Global unless your account uses an EU regional subuser — the EU host has no Marketing endpoints (contacts, lists, segments)",
	},
	{Name: "start_date", Type: core.ConnectionTypeString, Label: "Start Date", Placeholder: "2026-01-31", Required: true},
	{Name: "end_date", Type: core.ConnectionTypeString, Label: "End Date", Placeholder: "2026-01-31 — defaults to today"},
	{
		Name:  "aggregated_by",
		Type:  core.ConnectionTypeString,
		Label: "Aggregated By",
		Options: []core.ConnectionOption{
			{Name: "Day", Value: "day"},
			{Name: "Week", Value: "week"},
			{Name: "Month", Value: "month"},
		},
		Placeholder: "How to bucket the statistics — by day unless you choose otherwise",
	},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Stats"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	startDate, err := sendgrid.RequiredString("start_date", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	if !validDate(startDate) {
		return sendgrid.ErrorResult(fmt.Sprintf("start_date must be a date in YYYY-MM-DD format (got %q)", startDate)), nil
	}
	endDate := sendgrid.OptionalString("end_date", inputs)
	if endDate != "" && !validDate(endDate) {
		return sendgrid.ErrorResult(fmt.Sprintf("end_date must be a date in YYYY-MM-DD format (got %q)", endDate)), nil
	}

	query := url.Values{}
	query.Set("start_date", startDate)
	if endDate != "" {
		query.Set("end_date", endDate)
	}
	sendgrid.AddFilter(query, inputs, "aggregated_by", "aggregated_by")

	// The endpoint answers with a TOP-LEVEL ARRAY of {date, stats[]} buckets,
	// one per aggregation period.
	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/stats", query, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	items, ok := result.([]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid stats response shape"), nil
	}
	return sendgrid.ListResult(items, len(items), fmt.Sprintf("Retrieved stats for %d period(s)", len(items))), nil
}

// validDate accepts only a strict YYYY-MM-DD date — the stats endpoint takes
// dates, not datetimes, and Go's lenient parser would otherwise let "2026-1-5"
// through to a SendGrid 400.
func validDate(v string) bool {
	t, err := time.Parse("2006-01-02", v)
	return err == nil && t.Format("2006-01-02") == v
}

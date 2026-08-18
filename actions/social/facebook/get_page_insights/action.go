package facebook_get_page_insights

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	fb "flomation.app/automate/executor/actions/social/facebook"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Facebook Page Insights"
	Description  = "Get engagement and reach metrics for a Facebook Page"
	Website      = "https://www.flomation.co"
	Icon         = "facebook+pie-chart"
	Date         = "21/05/2026"
	Type         = core.ActionTypeAction
)

// defaultMetrics drops page_engaged_users, which no longer appears in Meta's
// current Page Insights reference, in favour of page_post_engagements, which
// does. Meta deprecated the reach/impressions family on 15 June 2026 in favour
// of Media Views and Media Viewers, so this set is a moving target — which is
// why it is a default rather than a hardcoded list.
const defaultMetrics = "page_impressions,page_post_engagements,page_fans"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Page Access Token", Placeholder: "${first_page_token}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret", Placeholder: "${secrets.facebook_app_secret}"},
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Page ID", Placeholder: "${first_page_id}", Required: true},
	// Metrics are configurable because Meta retires them on its own schedule,
	// independently of API version — the June 2026 retirement of the reach and
	// impressions family applied to ALL versions at once. A hardcoded list turns
	// every such retirement into a code change; an input lets an author move
	// without waiting for one.
	{Name: "metrics", Type: core.ConnectionTypeString, Label: "Metrics (comma-separated)", Placeholder: defaultMetrics},
	{Name: "period", Type: core.ConnectionTypeString, Label: "Period", Placeholder: "day",
		Options: []core.ConnectionOption{
			{Name: "Day", Value: "day"},
			{Name: "Week", Value: "week"},
			{Name: "28 Days", Value: "days_28"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
	{Name: "page_impressions", Type: core.ConnectionTypeInteger, Label: "Page Impressions"},
	{Name: "page_engaged_users", Type: core.ConnectionTypeInteger, Label: "Engaged Users"},
	{Name: "page_fans", Type: core.ConnectionTypeInteger, Label: "Total Followers"},
	{Name: "metrics", Type: core.ConnectionTypeObject, Label: "All Returned Metrics"},
	{Name: "missing_metrics", Type: core.ConnectionTypeObject, Label: "Requested But Not Returned"},
	{Name: "insights_json", Type: core.ConnectionTypeString, Label: "Full Insights JSON"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := fb.GetAccessToken(inputs)
	if err != nil {
		return fb.ErrorResult(err.Error()), nil
	}

	pageID := fb.OptionalString("page_id", inputs)
	if pageID == "" {
		return fb.ErrorResult("page_id is required"), nil
	}

	period := fb.OptionalString("period", inputs)
	if period == "" {
		period = "day"
	}

	metricList := fb.OptionalString("metrics", inputs)
	if metricList == "" {
		metricList = defaultMetrics
	}

	params := url.Values{
		"metric": {metricList},
		"period": {period},
	}

	appSecret := fb.GetAppSecret(inputs)
	endpoint := fmt.Sprintf("%s/%s/insights", fb.GraphAPIBase, pageID)
	resp, err := fb.ExecuteAPI(token, appSecret, "GET", endpoint, params)
	if err != nil {
		return fb.ErrorResult(fmt.Sprintf("failed to get insights: %v", err)), nil
	}

	if err := fb.CheckResponse(resp); err != nil {
		return fb.ErrorResult(err.Error()), nil
	}

	var result struct {
		Data []struct {
			Name   string `json:"name"`
			Values []struct {
				Value interface{} `json:"value"`
			} `json:"values"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return fb.ErrorResult(fmt.Sprintf("failed to parse insights: %v", err)), nil
	}

	metrics := make(map[string]int64)
	for _, d := range result.Data {
		if len(d.Values) > 0 {
			if v, ok := d.Values[len(d.Values)-1].Value.(float64); ok {
				metrics[d.Name] = int64(v)
			}
		}
	}

	insightsJSON, _ := json.Marshal(result.Data)

	// A metric Meta did not return reads as 0 from the map, which is
	// indistinguishable from a genuine zero — so a RETIRED metric silently
	// reports "Engaged: 0" on a busy page. Name the absentees instead.
	var missing []string
	for _, want := range strings.Split(metricList, ",") {
		want = strings.TrimSpace(want)
		if want == "" {
			continue
		}
		if _, ok := metrics[want]; !ok {
			missing = append(missing, want)
		}
	}

	parts := make([]string, 0, len(metrics))
	for _, want := range strings.Split(metricList, ",") {
		want = strings.TrimSpace(want)
		if v, ok := metrics[want]; ok {
			parts = append(parts, fmt.Sprintf("%s: %d", want, v))
		}
	}
	summary := strings.Join(parts, ", ")
	if summary == "" {
		summary = "No metrics returned"
	}
	if len(missing) > 0 {
		summary += fmt.Sprintf(" — NOT RETURNED by Meta: %s. These are reported as 0 below, which does not mean zero: Meta retires Page metrics across ALL API versions (the reach and impressions family went on 15 June 2026, replaced by Media Views/Media Viewers). Check the current Page Insights reference and set the Metrics input accordingly.",
			strings.Join(missing, ", "))
	}

	return fb.SuccessResult(
		summary,
		map[string]interface{}{
			"page_impressions":   metrics["page_impressions"],
			"page_engaged_users": metrics["page_engaged_users"],
			"page_fans":          metrics["page_fans"],
			"metrics":            metrics,
			"missing_metrics":    missing,
			"insights_json":      string(insightsJSON),
		},
	), nil
}

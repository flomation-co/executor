package facebook_get_page_insights

import (
	"encoding/json"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	fb "flomation.app/automate/executor/actions/social/facebook"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Facebook Page Insights"
	Description  = "Get engagement and reach metrics for a Facebook Page"
	Website      = "https://www.flomation.co"
	Icon         = "facebook"
	Date         = "21/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "Page Access Token", Placeholder: "${first_page_token}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeString, Label: "App Secret", Placeholder: "${secrets.facebook_app_secret}"},
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Page ID", Placeholder: "${first_page_id}", Required: true},
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

	params := url.Values{
		"metric": {"page_impressions,page_engaged_users,page_fans"},
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

	return fb.SuccessResult(
		fmt.Sprintf("Impressions: %d, Engaged: %d, Fans: %d",
			metrics["page_impressions"], metrics["page_engaged_users"], metrics["page_fans"]),
		map[string]interface{}{
			"page_impressions":  metrics["page_impressions"],
			"page_engaged_users": metrics["page_engaged_users"],
			"page_fans":         metrics["page_fans"],
			"insights_json":     string(insightsJSON),
		},
	), nil
}

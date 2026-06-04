package webflow_list_pages

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	webflow "flomation.app/automate/executor/actions/webflow"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Pages"
	Description  = "List all pages for a Webflow site"
	Website      = "https://www.flomation.co"
	Icon         = "webflow+list"
	Date         = "29/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_token",
		Type:        core.ConnectionTypeString,
		Label:       "Webflow API Token",
		Placeholder: "wfl_...",
		Required:    true,
	},
	{
		Name:        "site_id",
		Type:        core.ConnectionTypeString,
		Label:       "Site ID",
		Placeholder: "The Webflow site ID",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pages", Type: core.ConnectionTypeObject, Label: "Pages"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := webflow.GetAPIToken(inputs)
	if err != nil {
		return nil, err
	}

	siteID, err := webflow.RequiredString("site_id", inputs)
	if err != nil {
		return nil, err
	}

	status, body, err := webflow.ExecuteRequest(token, "GET", "/sites/"+siteID+"/pages", nil)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to list pages: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var resp struct {
		Pages []json.RawMessage `json:"pages"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err))
	}

	type pageRow struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Slug  string `json:"slug"`
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d page(s):\n", len(resp.Pages))
	var parsed []interface{}
	for _, raw := range resp.Pages {
		var row pageRow
		if err := json.Unmarshal(raw, &row); err == nil {
			fmt.Fprintf(&sb, "- %s (id: %s, slug: %s)\n", row.Title, row.ID, row.Slug)
		}
		var generic interface{}
		_ = json.Unmarshal(raw, &generic)
		parsed = append(parsed, generic)
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"pages":       parsed,
		"count":       len(resp.Pages),
		"success":     true,
		"error":       "",
	}, nil
}

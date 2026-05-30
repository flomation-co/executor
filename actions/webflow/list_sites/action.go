package webflow_list_sites

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
	Name         = "List Sites"
	Description  = "List all Webflow sites accessible with the provided API token"
	Website      = "https://www.flomation.co"
	Icon         = "globe"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "sites", Type: core.ConnectionTypeObject, Label: "Sites"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := webflow.GetAPIToken(inputs)
	if err != nil {
		return nil, err
	}

	status, body, err := webflow.ExecuteRequest(token, "GET", "/sites", nil)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to list sites: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var resp struct {
		Sites []json.RawMessage `json:"sites"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err))
	}

	type siteRow struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		ShortName   string `json:"shortName"`
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d site(s):\n", len(resp.Sites))
	var parsed []interface{}
	for _, raw := range resp.Sites {
		var row siteRow
		if err := json.Unmarshal(raw, &row); err == nil {
			fmt.Fprintf(&sb, "- %s (id: %s, short: %s)\n", row.DisplayName, row.ID, row.ShortName)
		}
		var generic interface{}
		_ = json.Unmarshal(raw, &generic)
		parsed = append(parsed, generic)
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"sites":       parsed,
		"count":       len(resp.Sites),
		"success":     true,
		"error":       "",
	}, nil
}

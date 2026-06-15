package gitlab_list_projects

import (
	"encoding/json"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab List Projects"
	Description  = "List projects accessible to the authenticated user"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+list"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Search by name"},
	{Name: "owned", Type: core.ConnectionTypeBoolean, Label: "Owned Only"},
	{Name: "membership", Type: core.ConnectionTypeBoolean, Label: "Member Of Only"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By", Options: []core.ConnectionOption{
		{Name: "Created At", Value: "created_at"},
		{Name: "Updated At", Value: "updated_at"},
		{Name: "Name", Value: "name"},
		{Name: "Last Activity", Value: "last_activity_at"},
	}},
	{Name: "per_page", Type: core.ConnectionTypeString, Label: "Per Page", Placeholder: "20 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "projects", Type: core.ConnectionTypeObject, Label: "Projects (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := gitlab.GetAccessToken(inputs)
	if err != nil {
		return nil, err
	}
	baseURL := gitlab.GetBaseURL(inputs)

	params := url.Values{}
	if v := gitlab.OptionalString("search", inputs); v != "" {
		params.Set("search", v)
	}
	if v := gitlab.OptionalBool("owned", inputs); v != nil && *v {
		params.Set("owned", "true")
	}
	if v := gitlab.OptionalBool("membership", inputs); v != nil && *v {
		params.Set("membership", "true")
	}
	if v := gitlab.OptionalString("order_by", inputs); v != "" {
		params.Set("order_by", v)
	}
	if v := gitlab.OptionalString("per_page", inputs); v != "" {
		params.Set("per_page", v)
	}

	path := "/projects"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := gitlab.ExecuteAPI(token, baseURL, "GET", path, nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to list projects: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var projects []interface{}
	if err := json.Unmarshal(resp.Body, &projects); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	// Build a readable summary for AI tool_result
	summary := fmt.Sprintf("Found %d project(s):\n", len(projects))
	for _, p := range projects {
		if pm, ok := p.(map[string]interface{}); ok {
			id := pm["id"]
			name := pm["name_with_namespace"]
			if name == nil {
				name = pm["name"]
			}
			webURL := pm["web_url"]
			summary += fmt.Sprintf("- ID: %v | %v | %v\n", id, name, webURL)
		}
	}

	return map[string]interface{}{
		"tool_result": summary,
		"projects":    projects,
		"count":       int64(len(projects)),
		"success":     true,
		"error":       "",
	}, nil
}

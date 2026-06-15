package gitlab_list_users

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
	Name         = "GitLab List Users"
	Description  = "List users on a GitLab instance with optional search"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+user"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Search by name, username, or email"},
	{Name: "active", Type: core.ConnectionTypeBoolean, Label: "Active Users Only"},
	{Name: "per_page", Type: core.ConnectionTypeString, Label: "Per Page", Placeholder: "20 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "users", Type: core.ConnectionTypeObject, Label: "Users (JSON)"},
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
	if v := gitlab.OptionalBool("active", inputs); v != nil && *v {
		params.Set("active", "true")
	}
	if v := gitlab.OptionalString("per_page", inputs); v != "" {
		params.Set("per_page", v)
	}

	path := "/users"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := gitlab.ExecuteAPI(token, baseURL, "GET", path, nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to list users: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var users []interface{}
	if err := json.Unmarshal(resp.Body, &users); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	summary := fmt.Sprintf("Found %d user(s):\n", len(users))
	for _, u := range users {
		if um, ok := u.(map[string]interface{}); ok {
			summary += fmt.Sprintf("- ID: %v | @%v | %v\n", um["id"], um["username"], um["name"])
		}
	}

	return map[string]interface{}{
		"tool_result": summary,
		"users":       users,
		"count":       int64(len(users)),
		"success":     true,
		"error":       "",
	}, nil
}

package jira_issue_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Issue"
	Description  = "Fetch a single Jira issue by its key. Optionally narrow the returned data to specific fields, or expand extra sections such as the changelog or rendered fields."
	Website      = "https://www.flomation.co"
	Icon         = "jira"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue key, e.g. SCRUM-1", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. summary,status,assignee"},
	{Name: "expand", Type: core.ConnectionTypeString, Label: "Expand", Placeholder: "Comma-separated sections to expand, e.g. changelog,renderedFields"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Issue Key"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Issue"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := jira.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	key, err := jira.RequiredString("issue_key", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	if v := jira.OptionalString("fields", inputs); v != "" {
		q.Set("fields", v)
	}
	if v := jira.OptionalString("expand", inputs); v != "" {
		q.Set("expand", v)
	}

	resp, err := jira.GetResource(auth, "/issue/"+key, q)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	out := jira.ResourceResult(resp, "")
	out["tool_result"] = fmt.Sprintf("Retrieved issue %s", out["id"])
	return out, nil
}

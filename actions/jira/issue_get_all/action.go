package jira_issue_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Search Issues"
	Description  = "Search Jira issues using a JQL query. Leave the query blank to list every issue, or narrow the search and choose which fields to return. Return everything, or cap the results to a limit."
	Website      = "https://www.flomation.co"
	Icon         = "jira+magnifying-glass"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "jql", Type: core.ConnectionTypeText, Label: "JQL Query", Placeholder: `e.g. project = SCRUM AND status = "In Progress" ORDER BY created DESC`},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. summary,status,assignee"},
	{Name: "expand", Type: core.ConnectionTypeString, Label: "Expand", Placeholder: "Comma-separated sections to expand, e.g. changelog,renderedFields"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Fetch every matching issue"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum issues to return when not returning all"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Issues"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := jira.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	jql := jira.OptionalString("jql", inputs)
	fields := jira.OptionalString("fields", inputs)
	expand := jira.StringList("expand", inputs)
	returnAll := jira.OptionalBool("return_all", inputs)
	limit, _ := jira.OptionalInt("limit", inputs)

	items, err := jira.SearchJQL(auth, jql, fields, expand, limit, returnAll)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	return jira.ListResult(items, len(items), fmt.Sprintf("Found %d issues", len(items))), nil
}

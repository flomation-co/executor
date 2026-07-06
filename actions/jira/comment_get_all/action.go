package jira_comment_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Many Comments"
	Description  = "List the comments on a Jira issue. Enter the issue key (e.g. SCRUM-1) and either return everything or cap the number returned. Optionally order by newest or oldest first."
	Website      = "https://www.flomation.co"
	Icon         = "jira+comments"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue to list comments for, e.g. SCRUM-1", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every comment (ignores Limit)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum number of comments to return (1-100)"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By", Placeholder: "Sort order for the comments", Options: []core.ConnectionOption{
		{Name: "Newest first", Value: "-created"},
		{Name: "Oldest first", Value: "created"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Comments"},
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
	key, err := jira.RequiredString("issue_key", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	returnAll := jira.OptionalBool("return_all", inputs)
	limit, _ := jira.OptionalInt("limit", inputs)

	q := url.Values{}
	if v := jira.OptionalString("order_by", inputs); v != "" {
		q.Set("orderBy", v)
	}

	items, total, err := jira.ListOffset(auth, "/issue/"+key+"/comment", "comments", q, 0, limit, returnAll)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	return jira.ListResult(items, total, fmt.Sprintf("Fetched %d comments", len(items))), nil
}

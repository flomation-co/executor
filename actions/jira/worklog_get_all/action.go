package jira_worklog_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Many Worklogs"
	Description  = "List the worklog entries logged against a Jira issue. Return every entry, or cap the count with a limit. Each entry includes who logged it, the time spent and any comment."
	Website      = "https://www.flomation.co"
	Icon         = "jira+clock"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue to list worklogs for, e.g. SCRUM-1", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every worklog rather than a single page"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max worklogs to return when not returning all (default 50, max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Worklogs"},
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

	items, total, err := jira.ListOffset(auth, "/issue/"+key+"/worklog", "worklogs", nil, 0, limit, returnAll)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	return jira.ListResult(items, total, fmt.Sprintf("Retrieved %d worklog(s) for %s", len(items), key)), nil
}

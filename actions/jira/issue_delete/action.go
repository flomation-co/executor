package jira_issue_delete

import (
	"net/url"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Delete Issue"
	Description  = "Permanently delete a Jira issue by its key. Optionally delete the issue's subtasks along with it. This cannot be undone."
	Website      = "https://www.flomation.co"
	Icon         = "jira+trash"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue key to delete, e.g. SCRUM-1", Required: true},
	{Name: "delete_subtasks", Type: core.ConnectionTypeBoolean, Label: "Delete Subtasks", Placeholder: "Also delete the issue's subtasks"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Issue Key"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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
	if jira.OptionalBool("delete_subtasks", inputs) {
		q.Set("deleteSubtasks", "true")
	}

	if err := jira.DeleteResource(auth, "/issue/"+key, q); err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	return jira.SuccessResult(key, map[string]interface{}{}, "Deleted issue "+key), nil
}

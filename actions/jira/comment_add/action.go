package jira_comment_add

import (
	"fmt"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Add Comment"
	Description  = "Add a comment to a Jira issue. Enter the issue key (e.g. SCRUM-1) and the comment text — it is posted as a plain-text comment on the issue."
	Website      = "https://www.flomation.co"
	Icon         = "jira+comment"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue to comment on, e.g. SCRUM-1", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Comment", Placeholder: "The comment text (plain text)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Comment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Comment"},
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
	body, err := jira.RequiredString("body", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	obj, err := jira.CreateResource(auth, "/issue/"+key+"/comment", map[string]interface{}{"body": body})
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	out := jira.ResourceResult(obj, "")
	out["tool_result"] = fmt.Sprintf("Added comment %s", out["id"])
	return out, nil
}

package jira_comment_update

import (
	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update Comment"
	Description  = "Change the text of an existing comment on a Jira issue. Enter the issue key (e.g. SCRUM-1), the comment ID, and the new comment text."
	Website      = "https://www.flomation.co"
	Icon         = "jira+pen"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue the comment belongs to, e.g. SCRUM-1", Required: true},
	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment ID", Placeholder: "The ID of the comment to update", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Comment", Placeholder: "The new comment text (plain text)", Required: true},
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
	commentID, err := jira.RequiredString("comment_id", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	body, err := jira.RequiredString("body", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	obj, err := jira.PutResource(auth, "/issue/"+key+"/comment/"+commentID, map[string]interface{}{"body": body})
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	return jira.ResourceResult(obj, "Updated comment "+commentID), nil
}

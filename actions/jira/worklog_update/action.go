package jira_worklog_update

import (
	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update Worklog"
	Description  = "Change an existing worklog entry on a Jira issue. Update the time spent, the comment or when the work started — supply only the fields you want to change. Returns the updated worklog."
	Website      = "https://www.flomation.co"
	Icon         = "jira+pen"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue the worklog belongs to, e.g. SCRUM-1", Required: true},
	{Name: "worklog_id", Type: core.ConnectionTypeString, Label: "Worklog ID", Placeholder: "The ID of the worklog entry to update", Required: true},
	{Name: "time_spent", Type: core.ConnectionTypeString, Label: "Time Spent", Placeholder: "New time spent, e.g. 2h"},
	{Name: "comment", Type: core.ConnectionTypeText, Label: "Comment", Placeholder: "New comment (plain text)"},
	{Name: "started", Type: core.ConnectionTypeString, Label: "Started", Placeholder: "When work started, e.g. 2026-07-06T09:00:00.000+0000"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"visibility":{"type":"group","value":"jira-developers"}}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Worklog ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Worklog"},
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
	worklogID, err := jira.RequiredString("worklog_id", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	jira.SetIfPresent(body, inputs, "timeSpent", "time_spent")
	jira.SetIfPresent(body, inputs, "comment", "comment")
	jira.SetIfPresent(body, inputs, "started", "started")
	if err := jira.MergeAdditionalFields(body, inputs); err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	if len(body) == 0 {
		return jira.ErrorResult("nothing to update"), nil
	}

	obj, err := jira.PutResource(auth, "/issue/"+key+"/worklog/"+worklogID, body)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	return jira.ResourceResult(obj, "Updated worklog "+worklogID), nil
}

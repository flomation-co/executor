package jira_worklog_get

import (
	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Worklog"
	Description  = "Fetch a single worklog entry from a Jira issue by its issue key and worklog ID. Returns the full worklog, including who logged it, the time spent and any comment."
	Website      = "https://www.flomation.co"
	Icon         = "jira+clock"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue the worklog belongs to, e.g. SCRUM-1", Required: true},
	{Name: "worklog_id", Type: core.ConnectionTypeString, Label: "Worklog ID", Placeholder: "The ID of the worklog entry to fetch", Required: true},
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

	obj, err := jira.GetResource(auth, "/issue/"+key+"/worklog/"+worklogID, nil)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	return jira.ResourceResult(obj, "Fetched worklog "+worklogID), nil
}

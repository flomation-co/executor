package jira_attachment_remove

import (
	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Delete Attachment"
	Description  = "Permanently delete a Jira attachment by its ID. This removes the file from its issue and cannot be undone."
	Website      = "https://www.flomation.co"
	Icon         = "jira+trash"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "attachment_id", Type: core.ConnectionTypeString, Label: "Attachment ID", Placeholder: "The attachment's numeric ID, e.g. 10001", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment ID"},
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
	id, err := jira.RequiredString("attachment_id", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	if err := jira.DeleteResource(auth, "/attachment/"+id, nil); err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	return jira.SuccessResult(id, map[string]interface{}{}, "Deleted attachment "+id), nil
}

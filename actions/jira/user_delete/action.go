package jira_user_delete

import (
	"net/url"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Delete User"
	Description  = "Remove a user from your Atlassian site by their account ID. This deletes the user from the site — requires organisation-admin permission."
	Website      = "https://www.flomation.co"
	Icon         = "jira+user-minus"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account ID", Placeholder: "The account ID of the user to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Account ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := jira.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	accountID, err := jira.RequiredString("account_id", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	q := url.Values{"accountId": {accountID}}
	if err := jira.DeleteResource(auth, "/user", q); err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	return jira.SuccessResult(accountID, map[string]interface{}{}, "Deleted user "+accountID), nil
}

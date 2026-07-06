package jira_user_get

import (
	"net/url"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get User"
	Description  = "Look up a single Atlassian user by their account ID and return their profile. Optionally expand extra details (e.g. groups, applicationRoles) via the Expand field."
	Website      = "https://www.flomation.co"
	Icon         = "jira+user"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account ID", Placeholder: "The account ID of the user to retrieve", Required: true},
	{Name: "expand", Type: core.ConnectionTypeString, Label: "Expand", Placeholder: "Comma-separated extra details, e.g. groups,applicationRoles"},
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
	if expand := jira.OptionalString("expand", inputs); expand != "" {
		q.Set("expand", expand)
	}

	obj, err := jira.GetResource(auth, "/user", q)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	return jira.SuccessResult(accountID, obj, "Retrieved user "+accountID), nil
}

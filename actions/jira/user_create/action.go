package jira_user_create

import (
	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create User"
	Description  = "Invite a new user to your Atlassian site by email address. Optionally set a display name and choose which products to grant access to (leave products empty to invite the user to every product). Requires organisation-admin permission on the site."
	Website      = "https://www.flomation.co"
	Icon         = "jira+user-plus"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "email_address", Type: core.ConnectionTypeString, Label: "Email Address", Placeholder: "The email address of the user to invite", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "The name shown for the user, e.g. Jane Doe"},
	{Name: "products", Type: core.ConnectionTypeString, Label: "Products", Placeholder: "Comma-separated product keys (e.g. jira-software,jira-servicedesk). Leave empty to invite to all products."},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"key":"value"}`},
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
	emailAddr, err := jira.RequiredString("email_address", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"emailAddress": emailAddr}
	jira.SetIfPresent(body, inputs, "displayName", "display_name")
	// An empty products array invites the user to every product on the site;
	// a populated list restricts the invite to those products.
	if products := jira.StringList("products", inputs); len(products) > 0 {
		body["products"] = products
	} else {
		body["products"] = []string{}
	}
	if err := jira.MergeAdditionalFields(body, inputs); err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	obj, err := jira.CreateResource(auth, "/user", body)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	accountID, _ := obj["accountId"].(string)
	return jira.SuccessResult(accountID, obj, "Created user "+emailAddr), nil
}

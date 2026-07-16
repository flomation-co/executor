package azure_entra_user_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Create User"
	Description  = "Create a Microsoft Entra ID user with an initial password. Set any other Graph property (givenName, surname, jobTitle, department, usageLocation, …) via Additional Fields. Requires the User.ReadWrite.All application permission (admin-consented)."
	Website      = "https://www.flomation.co"
	Icon         = "azure+user-plus"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Jane Doe (max 256 characters)", Required: true},
	{Name: "user_principal_name", Type: core.ConnectionTypeString, Label: "User Principal Name", Placeholder: "jane.doe@your-tenant.onmicrosoft.com", Required: true},
	{Name: "mail_nickname", Type: core.ConnectionTypeString, Label: "Mail Nickname", Placeholder: "jane.doe — local part only, no @ (max 64 characters)", Required: true},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "Initial password — must meet the tenant's password policy", Required: true},
	{Name: "account_enabled", Type: core.ConnectionTypeBoolean, Label: "Account Enabled", Placeholder: "On (default): the user can sign in immediately", Value: true},
	{Name: "force_change_password", Type: core.ConnectionTypeBoolean, Label: "Force Password Change", Placeholder: "Require a new password at next sign-in"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"givenName":"Jane","surname":"Doe","jobTitle":"Engineer","department":"R&D","usageLocation":"GB"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := entra.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	displayName, err := entra.RequiredString("display_name", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	upn, err := entra.RequiredString("user_principal_name", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	nickname, err := entra.RequiredString("mail_nickname", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	password, err := entra.RequiredString("password", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	// Validate client-side so a bad value fails with a named field instead of
	// an opaque Graph 400 (mirrors n8n's preSend checks).
	if err := entra.ValidateDisplayName(displayName); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if err := entra.ValidateUPN(upn); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if err := entra.ValidateMailNickname(nickname); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"displayName":       displayName,
		"userPrincipalName": upn,
		"mailNickname":      nickname,
		"accountEnabled":    entra.BoolOrDefault("account_enabled", inputs, true),
		"passwordProfile": map[string]interface{}{
			"password":                      password,
			"forceChangePasswordNextSignIn": entra.OptionalBool("force_change_password", inputs),
		},
	}
	if err := entra.MergeAdditionalFields(body, inputs); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	resp, err := entra.ExecuteAPI(flow, auth, "POST", "/users", body)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if err := entra.CheckResponse(resp); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	obj, err := entra.Decode(resp)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	out := entra.ResourceResult(obj, "")
	out["tool_result"] = fmt.Sprintf("Created user %s (%s)", upn, out["id"])
	return out, nil
}

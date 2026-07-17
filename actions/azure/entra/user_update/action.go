package azure_entra_user_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Update User"
	Description  = "Update a Microsoft Entra ID user. Common toggles are first-class; any other Graph property goes in Update Fields (which wins on a key set both ways). SharePoint-backed personal properties (aboutMe, birthday, skills, …) cannot be set app-only and are not supported. Requires the User.ReadWrite.All application permission."
	Website      = "https://www.flomation.co"
	Icon         = "azure+pen"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID or UPN", Placeholder: "Object ID (GUID) or jane.doe@your-tenant.onmicrosoft.com", Required: true},
	{Name: "account_enabled", Type: core.ConnectionTypeBoolean, Label: "Account Enabled", Placeholder: "Tick to enable / untick to disable; leave untouched to keep the current state"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name"},
	{Name: "job_title", Type: core.ConnectionTypeString, Label: "Job Title"},
	{Name: "department", Type: core.ConnectionTypeString, Label: "Department"},
	{Name: "update_fields", Type: core.ConnectionTypeObject, Label: "Update Fields (JSON)", Placeholder: `{"givenName":"Jane","usageLocation":"GB"} — any Graph user property`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Applied Fields"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := entra.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	userID, err := entra.RequiredString("user_id", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	entra.SetBoolIfSet(body, inputs, "accountEnabled", "account_enabled")
	if v := entra.OptionalString("display_name", inputs); v != "" {
		if err := entra.ValidateDisplayName(v); err != nil {
			return entra.ErrorResult(err.Error()), nil
		}
		body["displayName"] = v
	}
	entra.SetIfPresent(body, inputs, "jobTitle", "job_title")
	entra.SetIfPresent(body, inputs, "department", "department")
	// update_fields is merged last so the raw JSON wins on a key set both ways
	// (the additional_fields precedence convention).
	if err := entra.MergeObjectInput(body, inputs, "update_fields"); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if len(body) == 0 {
		return entra.ErrorResult("nothing to update — set update_fields or one of the convenience fields"), nil
	}

	resp, err := entra.ExecuteAPI(flow, auth, "PATCH", "/users/"+url.PathEscape(userID), body)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if err := entra.CheckResponse(resp); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	// PATCH /users returns 204 No Content — echo the applied fields.
	return entra.EchoResult(userID, body, fmt.Sprintf("Updated user %s (%d field(s))", userID, len(body))), nil
}

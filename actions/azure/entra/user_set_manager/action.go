package azure_entra_user_set_manager

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Set Manager"
	Description  = "Assign a user's manager — the core HR-driven provisioning step. Requires the User.ReadWrite.All application permission."
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
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID or UPN", Placeholder: "The user whose manager is being set", Required: true},
	{Name: "manager_id", Type: core.ConnectionTypeString, Label: "Manager (User ID)", Placeholder: "Object ID (GUID) of the manager", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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
	managerID, err := entra.RequiredString("manager_id", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	// Manager assignment is a $ref PUT (replace-the-reference semantics), not
	// a POST — the binding URL must be absolute on the same Graph host.
	body := map[string]interface{}{
		"@odata.id": auth.BaseURL() + "/users/" + url.PathEscape(managerID),
	}
	resp, err := entra.ExecuteAPI(flow, auth, "PUT", "/users/"+url.PathEscape(userID)+"/manager/$ref", body)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if err := entra.CheckResponse(resp); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	echo := map[string]interface{}{"updated": true, "user_id": userID, "manager_id": managerID}
	return entra.EchoResult(userID, echo, fmt.Sprintf("Set manager of %s to %s", userID, managerID)), nil
}

package azure_entra_user_get_manager

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Get Manager"
	Description  = "Get a user's manager. A user with no manager assigned fails softly with a clear message, so a flow can branch on it. Requires the User.Read.All application permission."
	Website      = "https://www.flomation.co"
	Icon         = "azure+user"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID or UPN", Placeholder: "Object ID (GUID) or jane.doe@your-tenant.onmicrosoft.com", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Manager ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Manager"},
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

	resp, err := entra.ExecuteAPI(flow, auth, "GET", "/users/"+url.PathEscape(userID)+"/manager", nil)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	// Graph 404s both a missing user and an unset manager; the common case in
	// a flow is the latter, so phrase it that way rather than as a lookup bug.
	if resp.StatusCode == http.StatusNotFound {
		return entra.ErrorResult(fmt.Sprintf("No manager is set for user %s (or the user does not exist)", userID)), nil
	}
	if err := entra.CheckResponse(resp); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	obj, err := entra.Decode(resp)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	out := entra.ResourceResult(obj, "")
	out["tool_result"] = fmt.Sprintf("Manager of %s is %v (%v)", userID, obj["displayName"], out["id"])
	return out, nil
}

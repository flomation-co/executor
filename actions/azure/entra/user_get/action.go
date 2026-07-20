package azure_entra_user_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Get User"
	Description  = "Get one Microsoft Entra ID user by object ID or userPrincipalName. Returns a rich default property set (Graph only returns a handful without an explicit $select); narrow it with Select Fields. Requires the User.Read.All application permission."
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
	{Name: "select", Type: core.ConnectionTypeString, Label: "Select Fields", Placeholder: "Comma-separated properties, e.g. id,displayName,mail — leave blank for the rich default set"},
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
	userID, err := entra.RequiredString("user_id", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	sel := entra.OptionalString("select", inputs)
	if sel == "" {
		sel = entra.DefaultUserSelect
	}
	q := url.Values{}
	q.Set("$select", sel)

	resp, err := entra.ExecuteAPI(flow, auth, "GET", "/users/"+url.PathEscape(userID)+"?"+q.Encode(), nil)
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
	label := userID
	if s, ok := obj["userPrincipalName"].(string); ok && s != "" {
		label = s
	}
	out["tool_result"] = fmt.Sprintf("Fetched user %s", label)
	return out, nil
}

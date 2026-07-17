package azure_entra_group_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Get Group"
	Description  = "Get one Microsoft Entra ID group by object ID. Narrow the returned properties with Select Fields. Requires the Group.Read.All application permission."
	Website      = "https://www.flomation.co"
	Icon         = "azure+user-group"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group ID", Placeholder: "Group object ID (GUID)", Required: true},
	{Name: "select", Type: core.ConnectionTypeString, Label: "Select Fields", Placeholder: "Comma-separated properties, e.g. id,displayName,visibility — leave blank for Graph's default set"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Group"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := entra.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	groupID, err := entra.RequiredString("group_id", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	path := "/groups/" + url.PathEscape(groupID)
	if sel := entra.OptionalString("select", inputs); sel != "" {
		q := url.Values{}
		q.Set("$select", sel)
		path += "?" + q.Encode()
	}
	resp, err := entra.ExecuteAPI(flow, auth, "GET", path, nil)
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
	label := groupID
	if s, ok := obj["displayName"].(string); ok && s != "" {
		label = s
	}
	out["tool_result"] = fmt.Sprintf("Fetched group %s", label)
	return out, nil
}

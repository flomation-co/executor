package azure_entra_group_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Delete Group"
	Description  = "Delete a Microsoft Entra ID group. A Microsoft 365 group moves to the directory recycle bin for 30 days (restorable with Restore Deleted Object); a Security group is deleted permanently. Requires the Group.ReadWrite.All application permission."
	Website      = "https://www.flomation.co"
	Icon         = "azure+trash"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group ID", Placeholder: "Group object ID (GUID)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group ID"},
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
	groupID, err := entra.RequiredString("group_id", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	resp, err := entra.ExecuteAPI(flow, auth, "DELETE", "/groups/"+url.PathEscape(groupID), nil)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if err := entra.CheckResponse(resp); err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	echo := map[string]interface{}{"deleted": true, "group_id": groupID}
	return entra.EchoResult(groupID, echo, fmt.Sprintf("Deleted group %s", groupID)), nil
}

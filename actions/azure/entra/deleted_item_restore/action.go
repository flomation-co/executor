package azure_entra_deleted_item_restore

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Restore Deleted Object"
	Description  = "Restore a soft-deleted user or Microsoft 365 group from the directory recycle bin (objects stay restorable for 30 days after deletion). Requires the User.ReadWrite.All (users) or Group.ReadWrite.All (groups) application permission."
	Website      = "https://www.flomation.co"
	Icon         = "azure+rotate-left"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "object_id", Type: core.ConnectionTypeString, Label: "Deleted Object ID", Placeholder: "Object ID (GUID) of the deleted user or group", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Restored Object"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := entra.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	objectID, err := entra.RequiredString("object_id", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	path := "/directory/deletedItems/" + url.PathEscape(objectID) + "/restore"
	resp, err := entra.ExecuteAPI(flow, auth, "POST", path, nil)
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
	out["tool_result"] = fmt.Sprintf("Restored %v (%v)", obj["displayName"], out["id"])
	return out, nil
}

package azure_entra_user_add_to_group

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Add User to Group"
	Description  = "Add one user to a group's members. Adding a user who is already a member fails softly with a clear message. To add several users at once, use Add Group Members. Requires the GroupMember.ReadWrite.All application permission."
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
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group ID", Placeholder: "Group object ID (GUID)", Required: true},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID", Placeholder: "User object ID (GUID)", Required: true},
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
	groupID, err := entra.RequiredString("group_id", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	userID, err := entra.RequiredString("user_id", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	// The $ref binding must be an absolute directoryObjects URL on the SAME
	// Graph host the request goes to (sovereign clouds included).
	body := map[string]interface{}{
		"@odata.id": auth.BaseURL() + "/directoryObjects/" + url.PathEscape(userID),
	}
	resp, err := entra.ExecuteAPI(flow, auth, "POST", "/groups/"+url.PathEscape(groupID)+"/members/$ref", body)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if err := entra.CheckResponse(resp); err != nil {
		// The already-a-member conflict is friendly-mapped by CheckResponse.
		return entra.ErrorResult(err.Error()), nil
	}
	echo := map[string]interface{}{"added": true, "group_id": groupID, "user_id": userID}
	return entra.EchoResult(userID, echo, fmt.Sprintf("Added user %s to group %s", userID, groupID)), nil
}

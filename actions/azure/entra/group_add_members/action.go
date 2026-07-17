package azure_entra_group_add_members

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Add Group Members"
	Description  = "Add many users to a group in one action. Graph accepts at most 20 member references per request, so longer lists are batched automatically (n8n adds one user at a time). A user who is already a member fails the whole batch — the error says how far it got. Requires the GroupMember.ReadWrite.All application permission."
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
	{Name: "user_ids", Type: core.ConnectionTypeString, Label: "User IDs", Placeholder: "Comma-separated user object IDs (batched 20 per request)", Required: true},
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
	raw, err := entra.RequiredString("user_ids", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	userIDs := entra.SplitCommaList(raw)
	if len(userIDs) == 0 {
		return entra.ErrorResult("user_ids must contain at least one user object ID"), nil
	}

	added := 0
	for _, chunk := range entra.ChunkStrings(userIDs, entra.ODataBindChunk) {
		binds := []interface{}{}
		for _, id := range chunk {
			binds = append(binds, auth.BaseURL()+"/directoryObjects/"+url.PathEscape(id))
		}
		body := map[string]interface{}{"members@odata.bind": binds}
		resp, err := entra.ExecuteAPI(flow, auth, "PATCH", "/groups/"+url.PathEscape(groupID), body)
		if err == nil {
			err = entra.CheckResponse(resp)
		}
		if err != nil {
			// Later batches can fail after earlier ones landed — say how far
			// the add got so the operator knows the group's actual state.
			return entra.ErrorResult(fmt.Sprintf("added %d of %d member(s) before the error: %s", added, len(userIDs), err.Error())), nil
		}
		added += len(chunk)
	}

	echo := map[string]interface{}{"added": added, "group_id": groupID, "user_ids": userIDs}
	batches := (len(userIDs) + entra.ODataBindChunk - 1) / entra.ODataBindChunk
	return entra.EchoResult(groupID, echo, fmt.Sprintf("Added %d member(s) to group %s in %d batch(es)", added, groupID, batches)), nil
}

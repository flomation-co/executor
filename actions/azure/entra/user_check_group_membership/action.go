package azure_entra_user_check_group_membership

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Check Group Membership"
	Description  = "Check whether a user is a member of the given groups (transitive — nested membership counts). Is Member is true when the user is in ANY of the listed groups; Member Of lists exactly which ones matched. Graph checks at most 20 group IDs per call, so longer lists are batched automatically. Requires the User.Read.All and GroupMember.Read.All application permissions."
	Website      = "https://www.flomation.co"
	Icon         = "azure+check"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID or UPN", Placeholder: "Object ID (GUID) or jane.doe@your-tenant.onmicrosoft.com", Required: true},
	{Name: "group_ids", Type: core.ConnectionTypeString, Label: "Group IDs", Placeholder: "Comma-separated group object IDs (checked in batches of 20)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Membership Check"},
	{Name: "member_of", Type: core.ConnectionTypeObject, Label: "Member Of (matched group IDs)"},
	{Name: "is_member", Type: core.ConnectionTypeBoolean, Label: "Is Member (of ANY listed group)"},
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
	raw, err := entra.RequiredString("group_ids", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	groupIDs := entra.SplitCommaList(raw)
	if len(groupIDs) == 0 {
		return entra.ErrorResult("group_ids must contain at least one group object ID"), nil
	}

	path := "/users/" + url.PathEscape(userID) + "/checkMemberGroups"
	matched := []interface{}{}
	for _, chunk := range entra.ChunkStrings(groupIDs, entra.ODataBindChunk) {
		resp, err := entra.ExecuteAPI(flow, auth, "POST", path, map[string]interface{}{"groupIds": chunk})
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
		if ids, ok := obj["value"].([]interface{}); ok {
			matched = append(matched, ids...)
		}
	}

	isMember := len(matched) > 0
	out := entra.ResourceResult(map[string]interface{}{
		"is_member": isMember,
		"member_of": matched,
		"checked":   len(groupIDs),
	}, fmt.Sprintf("User %s is a member of %d of the %d checked group(s)", userID, len(matched), len(groupIDs)))
	out["id"] = userID
	out["member_of"] = matched
	out["is_member"] = isMember
	return out, nil
}

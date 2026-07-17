package azure_entra_user_list_groups

import (
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: List User's Groups"
	Description  = "List the groups (and directory roles) a user is a member of. Turn on Transitive to include nested memberships — groups the user is in via other groups. Requires the User.Read.All and GroupMember.Read.All application permissions."
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
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID or UPN", Placeholder: "Object ID (GUID) or jane.doe@your-tenant.onmicrosoft.com", Required: true},
	{Name: "transitive", Type: core.ConnectionTypeBoolean, Label: "Include Transitive (nested) Memberships", Placeholder: "Also return groups the user belongs to through other groups"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (follow every page)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 999); ignored when Return All is on"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Groups"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	relation := "/memberOf"
	if entra.OptionalBool("transitive", inputs) {
		relation = "/transitiveMemberOf"
	}
	q := url.Values{}
	returnAll := entra.ApplyPaging(q, inputs)

	items, next, err := entra.ListAll(flow, auth, "/users/"+url.PathEscape(userID)+relation, q, returnAll)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	return entra.ListResult(items, entra.ListSummary("group", len(items), returnAll, next != "")), nil
}

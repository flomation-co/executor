package azure_entra_group_list_members

import (
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: List Group Members"
	Description  = "List a group's members with proper paging (n8n only offers a capped $expand on group get). Turn on Transitive to include members inherited through nested groups. Requires the GroupMember.Read.All application permission."
	Website      = "https://www.flomation.co"
	Icon         = "azure+people-group"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group ID", Placeholder: "Group object ID (GUID)", Required: true},
	{Name: "transitive", Type: core.ConnectionTypeBoolean, Label: "Include Transitive (nested) Members", Placeholder: "Also return members inherited through nested groups"},
	{Name: "select", Type: core.ConnectionTypeString, Label: "Select Fields", Placeholder: "Comma-separated properties, e.g. id,displayName,userPrincipalName"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (follow every page)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 999); ignored when Return All is on"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Members"},
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
	groupID, err := entra.RequiredString("group_id", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	relation := "/members"
	if entra.OptionalBool("transitive", inputs) {
		relation = "/transitiveMembers"
	}
	q := url.Values{}
	returnAll := entra.ApplyPaging(q, inputs)
	if v := entra.OptionalString("select", inputs); v != "" {
		q.Set("$select", v)
	}

	items, next, err := entra.ListAll(flow, auth, "/groups/"+url.PathEscape(groupID)+relation, q, returnAll)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	return entra.ListResult(items, entra.ListSummary("member", len(items), returnAll, next != "")), nil
}

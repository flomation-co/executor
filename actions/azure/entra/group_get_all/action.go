package azure_entra_group_get_all

import (
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Get Many Groups"
	Description  = "List Microsoft Entra ID groups with raw OData $filter/$search passthrough. Advanced queries (endsWith, filter on null, $search) work — ConsistencyLevel: eventual and $count=true are sent on every request. Requires the Group.Read.All application permission."
	Website      = "https://www.flomation.co"
	Icon         = "azure+list"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "filter", Type: core.ConnectionTypeString, Label: "Filter ($filter)", Placeholder: `startswith(displayName,'Sales') — raw OData filter; advanced operators supported`},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search ($search)", Placeholder: `"displayName:sales" — quoted property:value clauses`},
	{Name: "select", Type: core.ConnectionTypeString, Label: "Select Fields", Placeholder: "Comma-separated properties, e.g. id,displayName,mailNickname"},
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

	q := url.Values{}
	returnAll := entra.ApplyPaging(q, inputs)
	if v := entra.OptionalString("filter", inputs); v != "" {
		q.Set("$filter", v)
	}
	if v := entra.OptionalString("search", inputs); v != "" {
		q.Set("$search", v)
	}
	if v := entra.OptionalString("select", inputs); v != "" {
		q.Set("$select", v)
	}

	items, next, err := entra.ListAll(flow, auth, "/groups", q, returnAll)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	return entra.ListResult(items, entra.ListSummary("group", len(items), returnAll, next != "")), nil
}

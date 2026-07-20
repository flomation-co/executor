package azure_entra_subscribed_skus_get_all

import (
	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Get Subscribed SKUs"
	Description  = "List the licence SKUs the tenant has subscribed to — skuId, skuPartNumber, consumed vs available units. Feed the skuId GUIDs into Assign License. Requires the Organization.Read.All application permission."
	Website      = "https://www.flomation.co"
	Icon         = "azure+clipboard-list"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Subscribed SKUs"},
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

	// /subscribedSkus supports only $select — no paging, no $count, no
	// $filter — so this goes through the plain list path without the
	// advanced-query pair (Graph rejects unsupported params here).
	items, err := entra.ListSimple(flow, auth, "/subscribedSkus", nil)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	return entra.ListResult(items, entra.ListSummary("subscribed SKU", len(items), true, false)), nil
}

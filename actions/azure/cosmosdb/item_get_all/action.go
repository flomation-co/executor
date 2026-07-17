// Package azure_cosmosdb_item_get_all lists a container's items — a
// cross-partition feed read (no partition-key header), enveloped in
// {"Documents":[...]} and paginated by x-ms-continuation headers.
package azure_cosmosdb_item_get_all

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	cosmosdb "flomation.app/automate/executor/actions/azure/cosmosdb"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cosmos DB: List Items"
	Description  = "List the items in a container, across all partitions."
	Website      = "https://www.flomation.co"
	Icon         = "azure+list"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account_name", Type: core.ConnectionTypeString, Label: "Account Name", Placeholder: "mycosmosaccount", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{
		{Name: "Master Key", Value: "master_key"},
		{Name: "Microsoft Entra (service principal)", Value: "entra"},
	}},
	{Name: "master_key", Type: core.ConnectionTypeSecret, Label: "Master Key", Placeholder: "Primary or secondary key (base64) — Azure Portal ▸ your account ▸ Keys", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "master_key"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The service principal's client secret", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://localhost:8081 for the emulator — leave blank for https://{account}.documents.azure.com"},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip TLS verification — required for the Cosmos DB emulator's self-signed certificate"},

	{Name: "database", Type: core.ConnectionTypeString, Label: "Database", Placeholder: "The database ID", Required: true},
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container", Placeholder: "The container ID", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow every continuation token until all items are fetched"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum items per page — default 50, maximum 1000"},
	{Name: "continuation", Type: core.ConnectionTypeString, Label: "Continuation Token", Placeholder: "Resume from an earlier run's Next Continuation — leave blank to start at the first page"},
	{Name: "simplify", Type: core.ConnectionTypeBoolean, Label: "Simplify", Placeholder: "Strip Cosmos system properties (_rid, _etag, _ts, …) — on by default"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Items"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "request_charge", Type: core.ConnectionTypeString, Label: "Request Charge (RU)"},
	{Name: "next_continuation", Type: core.ConnectionTypeString, Label: "Next Continuation"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := cosmosdb.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	db, err := cosmosdb.RequiredString("database", inputs)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	coll, err := cosmosdb.RequiredString("container", inputs)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	limit, set := cosmosdb.OptionalInt("limit", inputs)
	returnAll := cosmosdb.OptionalBool("return_all", inputs)

	items, charge, next, err := cosmosdb.Feed(flow, auth, http.MethodGet, cosmosdb.DocsPath(db, coll), "docs", cosmosdb.CollRID(db, coll), "Documents", nil, nil, cosmosdb.ClampLimit(limit, set), returnAll, cosmosdb.OptionalString("continuation", inputs))
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	items = cosmosdb.SimplifyItems(items, cosmosdb.BoolDefaultTrue("simplify", inputs))
	return cosmosdb.ListResult(items, charge, next, returnAll, fmt.Sprintf("Fetched %d items from container %q", len(items), coll)), nil
}

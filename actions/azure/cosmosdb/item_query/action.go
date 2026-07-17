// Package azure_cosmosdb_item_query runs a parameterized SQL query against a
// container.
//
// Queries POST to the docs feed with Content-Type application/query+json and
// the x-ms-documentdb-isquery marker. Cross-partition by default; setting the
// Partition Key input scopes the query to one partition instead (cheaper).
// Results paginate through the same x-ms-continuation headers as feed reads —
// n8n's query operation has no pagination at all, silently truncating at the
// server's default page size.
package azure_cosmosdb_item_query

import (
	"encoding/json"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	cosmosdb "flomation.app/automate/executor/actions/azure/cosmosdb"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cosmos DB: Query Items"
	Description  = "Run a SQL query against a container, with named @parameters and pagination. Cross-partition by default; set Partition Key to scope to one partition."
	Website      = "https://www.flomation.co"
	Icon         = "azure+code"
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
	{Name: "query", Type: core.ConnectionTypeText, Label: "Query", Placeholder: "SELECT * FROM c WHERE c.status = @status", Required: true},
	{Name: "parameters", Type: core.ConnectionTypeObject, Label: "Parameters (JSON)", Placeholder: `{"@status":"open"} — names map to the @parameters in the query`},
	{Name: "partition_key", Type: core.ConnectionTypeString, Label: "Partition Key", Placeholder: "Scope the query to one partition — leave blank to query all partitions"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow every continuation token until all matching items are fetched"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum items per page — default 50, maximum 1000"},
	{Name: "continuation", Type: core.ConnectionTypeString, Label: "Continuation Token", Placeholder: "Resume from an earlier run's Next Continuation — the Query must be unchanged"},
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
	query, err := cosmosdb.RequiredString("query", inputs)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	params, err := cosmosdb.QueryParameters(inputs)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"query": query}
	if params != nil {
		body["parameters"] = params
	}
	payload, _ := json.Marshal(body)

	headers := map[string]string{
		"Content-Type":            "application/query+json",
		"x-ms-documentdb-isquery": "True",
	}
	if pk := cosmosdb.OptionalString("partition_key", inputs); pk != "" {
		headers["x-ms-documentdb-partitionkey"] = cosmosdb.PartitionKeyHeader(pk)
	} else {
		headers["x-ms-documentdb-query-enablecrosspartition"] = "True"
	}

	limit, set := cosmosdb.OptionalInt("limit", inputs)
	returnAll := cosmosdb.OptionalBool("return_all", inputs)

	// A resumed query MUST re-send the identical body — the continuation token
	// is only meaningful against the query that produced it.
	items, charge, next, err := cosmosdb.Feed(flow, auth, http.MethodPost, cosmosdb.DocsPath(db, coll), "docs", cosmosdb.CollRID(db, coll), "Documents", headers, payload, cosmosdb.ClampLimit(limit, set), returnAll, cosmosdb.OptionalString("continuation", inputs))
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	items = cosmosdb.SimplifyItems(items, cosmosdb.BoolDefaultTrue("simplify", inputs))
	return cosmosdb.ListResult(items, charge, next, returnAll, fmt.Sprintf("Query returned %d items from container %q", len(items), coll)), nil
}

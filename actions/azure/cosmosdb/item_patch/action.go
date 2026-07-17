// Package azure_cosmosdb_item_patch partially updates one item — a TRUE
// partial update via the Cosmos DB partial-document PATCH API, which n8n
// cannot do at all (its "update" is a destructive full replace).
//
// The wire shape is idiosyncratic: HTTP PATCH with Content-Type
// application/json_patch+json (underscores — NOT RFC 6902's json-patch+json)
// and a body of {"operations":[...]} where each operation is one of
// add/set/replace/remove/incr/move, at most ten per call, optionally guarded
// by a "condition" (a "FROM c WHERE ..." predicate evaluated server-side).
package azure_cosmosdb_item_patch

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
	Name         = "Cosmos DB: Patch Item"
	Description  = "Partially update an item — change only the named paths (add/set/replace/remove/incr/move, up to 10 operations), leaving the rest of the item untouched."
	Website      = "https://www.flomation.co"
	Icon         = "azure+pen"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

// allowedOps are the operations the Cosmos partial-update API accepts. RFC
// 6902 names it is NOT (no "test", no "copy"; "incr" and "set" are Cosmos
// extensions), so the set is validated client-side for a clear error.
var allowedOps = map[string]bool{
	"add": true, "set": true, "replace": true, "remove": true, "incr": true, "move": true,
}

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
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Placeholder: "The id of the item to patch", Required: true},
	{Name: "operations", Type: core.ConnectionTypeObject, Label: "Operations (JSON array)", Placeholder: `[{"op":"set","path":"/status","value":"done"},{"op":"incr","path":"/version","value":1}]`, Required: true},
	{Name: "condition", Type: core.ConnectionTypeString, Label: "Condition", Placeholder: "FROM c WHERE c.status = 'open' — only patch when this predicate holds"},
	{Name: "partition_key", Type: core.ConnectionTypeString, Label: "Partition Key", Placeholder: "The item's partition-key value — leave blank when the container is partitioned on /id"},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag (If-Match)", Placeholder: "Only patch if the item still has this _etag — fails cleanly when it changed"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Item ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Item"},
	{Name: "request_charge", Type: core.ConnectionTypeString, Label: "Request Charge (RU)"},
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
	itemID, err := cosmosdb.RequiredString("item_id", inputs)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	ops, err := cosmosdb.RequiredArray("operations", inputs)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if len(ops) == 0 {
		return cosmosdb.ErrorResult("operations must contain at least one operation"), nil
	}
	if len(ops) > 10 {
		return cosmosdb.ErrorResult(fmt.Sprintf("Cosmos DB allows at most 10 patch operations per call (got %d)", len(ops))), nil
	}
	for i, raw := range ops {
		op, ok := raw.(map[string]interface{})
		if !ok {
			return cosmosdb.ErrorResult(fmt.Sprintf(`operation %d must be an object like {"op":"set","path":"/status","value":"done"}`, i+1)), nil
		}
		name, _ := op["op"].(string)
		if !allowedOps[name] {
			return cosmosdb.ErrorResult(fmt.Sprintf("operation %d has op %q — must be one of add, set, replace, remove, incr, move", i+1, name)), nil
		}
	}

	body := map[string]interface{}{"operations": ops}
	cosmosdb.SetIfPresent(body, inputs, "condition", "condition")

	headers := map[string]string{"Content-Type": "application/json_patch+json"}
	pk, hasPK, err := cosmosdb.ResolvePointPartitionKey(flow, auth, inputs, db, coll, itemID)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if hasPK {
		headers["x-ms-documentdb-partitionkey"] = cosmosdb.PartitionKeyHeader(pk)
	}
	if etag := cosmosdb.OptionalString("etag", inputs); etag != "" {
		headers["If-Match"] = etag
	}

	payload, _ := json.Marshal(body)
	resp, err := cosmosdb.DoRequest(flow, auth, http.MethodPatch, cosmosdb.DocPath(db, coll, itemID), "docs", cosmosdb.DocRID(db, coll, itemID), headers, payload)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return cosmosdb.ErrorResult(fmt.Sprintf("item %q was not found in container %q — check the id and the partition key value", itemID, coll)), nil
	case http.StatusPreconditionFailed:
		return cosmosdb.ErrorResult(fmt.Sprintf("item %q was not patched — the etag no longer matches or the condition did not hold", itemID)), nil
	}
	if err := cosmosdb.CheckResponse(resp); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	obj, err := cosmosdb.DecodeObject(resp)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	return cosmosdb.ResourceResult(obj, cosmosdb.RequestCharge(resp), fmt.Sprintf("Patched item %q in container %q (%d operations)", itemID, coll, len(ops))), nil
}

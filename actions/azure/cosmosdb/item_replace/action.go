// Package azure_cosmosdb_item_replace replaces one item's entire body.
//
// This is a FULL replace: properties not present in the new body are removed
// from the stored item. (For a partial update that only touches named paths,
// use Patch Item.) The item's id comes from the Item ID input — any "id" in
// the body is overwritten with it, so the body cannot silently address a
// different item than the URL. An optional etag turns the replace into
// optimistic concurrency via If-Match: a 412 means someone changed the item
// since it was read, surfaced as a clean soft error.
package azure_cosmosdb_item_replace

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
	Name         = "Cosmos DB: Replace Item"
	Description  = "Replace an item's entire body — properties missing from the new body are REMOVED from the item. Use Patch Item for a partial update."
	Website      = "https://www.flomation.co"
	Icon         = "azure+rotate"
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
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Placeholder: "The id of the item to replace", Required: true},
	{Name: "item", Type: core.ConnectionTypeObject, Label: "New Item Body (JSON)", Placeholder: `{"status":"closed"} — becomes the item's ENTIRE body`, Required: true},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag (If-Match)", Placeholder: "Only replace if the item still has this _etag — fails cleanly when it changed"},
	{Name: "partition_key", Type: core.ConnectionTypeString, Label: "Partition Key", Placeholder: "The item's partition-key value — leave blank when it is a property of the body or the container is partitioned on /id"},
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
	item, err := cosmosdb.RequiredObject("item", inputs)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	// The URL addresses the item; the body must agree.
	item["id"] = itemID

	headers := map[string]string{}
	pk, hasPK, err := cosmosdb.ResolveBodyPartitionKey(flow, auth, inputs, db, coll, itemID, item)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if hasPK {
		headers["x-ms-documentdb-partitionkey"] = cosmosdb.PartitionKeyHeader(pk)
	}
	if etag := cosmosdb.OptionalString("etag", inputs); etag != "" {
		headers["If-Match"] = etag
	}

	payload, _ := json.Marshal(item)
	resp, err := cosmosdb.DoRequest(flow, auth, http.MethodPut, cosmosdb.DocPath(db, coll, itemID), "docs", cosmosdb.DocRID(db, coll, itemID), headers, payload)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return cosmosdb.ErrorResult(fmt.Sprintf("item %q was not found in container %q — check the id and the partition key value", itemID, coll)), nil
	case http.StatusPreconditionFailed:
		return cosmosdb.ErrorResult(fmt.Sprintf("item %q has been modified since it was read (etag mismatch) — re-read it and retry", itemID)), nil
	}
	if err := cosmosdb.CheckResponse(resp); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	obj, err := cosmosdb.DecodeObject(resp)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	return cosmosdb.ResourceResult(obj, cosmosdb.RequestCharge(resp), fmt.Sprintf("Replaced item %q in container %q", itemID, coll)), nil
}

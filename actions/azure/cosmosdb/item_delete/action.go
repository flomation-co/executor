// Package azure_cosmosdb_item_delete deletes one item by id. Cosmos returns
// 204 with no body; the result echoes what was deleted. Partition-key
// resolution is the same as Get Item's.
package azure_cosmosdb_item_delete

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	cosmosdb "flomation.app/automate/executor/actions/azure/cosmosdb"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cosmos DB: Delete Item"
	Description  = "Delete one item from a container by id. This cannot be undone."
	Website      = "https://www.flomation.co"
	Icon         = "azure+trash"
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
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Placeholder: "The id of the item to delete", Required: true},
	{Name: "partition_key", Type: core.ConnectionTypeString, Label: "Partition Key", Placeholder: "The item's partition-key value — leave blank when the container is partitioned on /id"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Item ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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

	headers := map[string]string{}
	pk, hasPK, err := cosmosdb.ResolvePointPartitionKey(flow, auth, inputs, db, coll, itemID)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if hasPK {
		headers["x-ms-documentdb-partitionkey"] = cosmosdb.PartitionKeyHeader(pk)
	}

	resp, err := cosmosdb.DoRequest(flow, auth, http.MethodDelete, cosmosdb.DocPath(db, coll, itemID), "docs", cosmosdb.DocRID(db, coll, itemID), headers, nil)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return cosmosdb.ErrorResult(fmt.Sprintf("item %q was not found in container %q — check the id and the partition key value", itemID, coll)), nil
	}
	if err := cosmosdb.CheckResponse(resp); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	return cosmosdb.ResourceResult(map[string]interface{}{"id": itemID, "container": coll, "database": db, "deleted": true},
		cosmosdb.RequestCharge(resp), fmt.Sprintf("Deleted item %q from container %q", itemID, coll)), nil
}

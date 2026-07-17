// Package azure_cosmosdb_item_create creates an item (document).
//
// Create is STRICT by default: an existing id in the same partition returns a
// clean 409 soft error. n8n hardcodes x-ms-documentdb-is-upsert and silently
// overwrites; here upsert is an explicit opt-in.
//
// The partition-key value is derived from the item body at the container's
// partition-key path (discovered once per execution), or from the Partition
// Key input, which is also injected into the body when the property is absent
// so the header and document can never disagree.
package azure_cosmosdb_item_create

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
	Name         = "Cosmos DB: Create Item"
	Description  = "Create an item in a container. Strict by default — creating an id that already exists fails unless Upsert is enabled, which overwrites it instead."
	Website      = "https://www.flomation.co"
	Icon         = "azure+plus"
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
	{Name: "item", Type: core.ConnectionTypeObject, Label: "Item (JSON)", Placeholder: `{"id":"order-1001","status":"open"} — must include id`, Required: true},
	{Name: "upsert", Type: core.ConnectionTypeBoolean, Label: "Upsert", Placeholder: "Overwrite the item if the id already exists — off by default (strict create)"},
	{Name: "partition_key", Type: core.ConnectionTypeString, Label: "Partition Key", Placeholder: "The item's partition-key value — leave blank when it is a property of the item itself"},
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
	item, err := cosmosdb.RequiredObject("item", inputs)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}

	upsert := cosmosdb.OptionalBool("upsert", inputs)
	id, _ := item["id"].(string)
	if !upsert && id == "" {
		return cosmosdb.ErrorResult(`the item must include an "id" property (a string) — or enable Upsert`), nil
	}

	headers := map[string]string{}
	if upsert {
		headers["x-ms-documentdb-is-upsert"] = "True"
	}
	pk, hasPK, err := cosmosdb.ResolveBodyPartitionKey(flow, auth, inputs, db, coll, id, item)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if hasPK {
		headers["x-ms-documentdb-partitionkey"] = cosmosdb.PartitionKeyHeader(pk)
	}

	payload, _ := json.Marshal(item)
	resp, err := cosmosdb.DoRequest(flow, auth, http.MethodPost, cosmosdb.DocsPath(db, coll), "docs", cosmosdb.CollRID(db, coll), headers, payload)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if resp.StatusCode == http.StatusConflict {
		return cosmosdb.ErrorResult(fmt.Sprintf("an item with id %q already exists in this partition — enable Upsert to overwrite it", id)), nil
	}
	if err := cosmosdb.CheckResponse(resp); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	obj, err := cosmosdb.DecodeObject(resp)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	verb := "Created"
	if upsert {
		verb = "Upserted"
	}
	createdID, _ := obj["id"].(string)
	return cosmosdb.ResourceResult(obj, cosmosdb.RequestCharge(resp), fmt.Sprintf("%s item %q in container %q", verb, createdID, coll)), nil
}

// Package azure_cosmosdb_container_replace changes a container's mutable
// definition — indexing policy and default TTL — after creation. n8n can only
// create/delete/get containers; this is one of the exceeds.
//
// The replace is read-modify-write: PUT requires the FULL definition and the
// partition key must be re-sent byte-for-byte unchanged (it is immutable), and
// a definition PUT with fields omitted silently resets them — so the current
// definition is fetched, the requested changes overlaid, and the merged result
// written back. Only system (underscore) properties are stripped from the
// round trip.
package azure_cosmosdb_container_replace

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
	Name         = "Cosmos DB: Replace Container"
	Description  = "Update a container's indexing policy and/or default item TTL. The partition key cannot be changed after creation."
	Website      = "https://www.flomation.co"
	Icon         = "azure+pen"
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
	{Name: "indexing_policy", Type: core.ConnectionTypeObject, Label: "Indexing Policy (JSON)", Placeholder: `{"indexingMode":"consistent","includedPaths":[{"path":"/*"}]} — replaces the current policy`},
	{Name: "default_ttl", Type: core.ConnectionTypeInteger, Label: "Default TTL (seconds)", Placeholder: "Expire items after this many seconds — -1 keeps items forever but lets per-item ttl work"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Container ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Container"},
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

	changes := map[string]interface{}{}
	if err := cosmosdb.SetJSONIfPresent(changes, inputs, "indexingPolicy", "indexing_policy"); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if err := cosmosdb.SetIntIfPresent(changes, inputs, "defaultTtl", "default_ttl"); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if len(changes) == 0 {
		return cosmosdb.ErrorResult("nothing to change — set Indexing Policy and/or Default TTL"), nil
	}

	// Read the current definition; the PUT must carry everything we are not
	// changing (partition key included) or the server resets/rejects it.
	resp, err := cosmosdb.DoRequest(flow, auth, http.MethodGet, cosmosdb.CollPath(db, coll), "colls", cosmosdb.CollRID(db, coll), nil, nil)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return cosmosdb.ErrorResult(fmt.Sprintf("container %q was not found in database %q", coll, db)), nil
	}
	if err := cosmosdb.CheckResponse(resp); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	current, err := cosmosdb.DecodeObject(resp)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}

	body := cosmosdb.Simplify(current)
	for k, v := range changes {
		body[k] = v
	}

	payload, _ := json.Marshal(body)
	resp, err = cosmosdb.DoRequest(flow, auth, http.MethodPut, cosmosdb.CollPath(db, coll), "colls", cosmosdb.CollRID(db, coll), nil, payload)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if err := cosmosdb.CheckResponse(resp); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	obj, err := cosmosdb.DecodeObject(resp)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	// The service forbids changing the partition key on a replace, so the path
	// itself is stable — but the name may have been dropped and recreated
	// earlier in this run, which is what would have staled the cache.
	cosmosdb.InvalidatePartitionKeyPath(auth, db, coll)
	return cosmosdb.ResourceResult(obj, cosmosdb.RequestCharge(resp), fmt.Sprintf("Replaced definition of container %q", coll)), nil
}

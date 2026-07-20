// Package azure_cosmosdb_container_create creates a container (collection).
//
// The partition-key path defaults to /id — the safest choice for small
// workloads and the one every other cosmosdb action can auto-derive a
// partition-key value for. Unique-key constraints are one path each: "a,b"
// becomes two independent constraints, not one compound key.
package azure_cosmosdb_container_create

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	cosmosdb "flomation.app/automate/executor/actions/azure/cosmosdb"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cosmos DB: Create Container"
	Description  = "Create a container in a database — set the partition key path, and optionally an indexing policy, item TTL, unique keys, and provisioned throughput (manual RU/s or autoscale)."
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
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container", Placeholder: "The ID of the container to create", Required: true},
	{Name: "partition_key_path", Type: core.ConnectionTypeString, Label: "Partition Key Path", Placeholder: "/id — the item property that partitions the container, e.g. /category"},
	{Name: "indexing_policy", Type: core.ConnectionTypeObject, Label: "Indexing Policy (JSON)", Placeholder: `{"indexingMode":"consistent","includedPaths":[{"path":"/*"}]}`},
	{Name: "default_ttl", Type: core.ConnectionTypeInteger, Label: "Default TTL (seconds)", Placeholder: "Expire items after this many seconds — -1 keeps items forever but lets per-item ttl work"},
	{Name: "unique_key_paths", Type: core.ConnectionTypeString, Label: "Unique Key Paths", Placeholder: "/email, /employeeId — comma-separated, one constraint each"},
	{Name: "throughput", Type: core.ConnectionTypeInteger, Label: "Throughput (RU/s)", Placeholder: "Dedicated manual throughput, minimum 400 — leave blank for the database default"},
	{Name: "autoscale_max", Type: core.ConnectionTypeInteger, Label: "Autoscale Max (RU/s)", Placeholder: "Dedicated autoscale maximum, minimum 1000 — mutually exclusive with Throughput"},
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
	headers, err := cosmosdb.ThroughputHeaders(inputs)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}

	pkPath := cosmosdb.OptionalString("partition_key_path", inputs)
	if pkPath == "" {
		pkPath = "/id"
	}
	if !strings.HasPrefix(pkPath, "/") {
		return cosmosdb.ErrorResult(fmt.Sprintf("partition_key_path must start with /, e.g. /category (got %q)", pkPath)), nil
	}

	body := map[string]interface{}{
		"id": coll,
		"partitionKey": map[string]interface{}{
			"paths":   []string{pkPath},
			"kind":    "Hash",
			"version": 2,
		},
	}
	if err := cosmosdb.SetJSONIfPresent(body, inputs, "indexingPolicy", "indexing_policy"); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if err := cosmosdb.SetIntIfPresent(body, inputs, "defaultTtl", "default_ttl"); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if raw := cosmosdb.OptionalString("unique_key_paths", inputs); raw != "" {
		keys := []map[string]interface{}{}
		for _, p := range strings.Split(raw, ",") {
			if p = strings.TrimSpace(p); p != "" {
				keys = append(keys, map[string]interface{}{"paths": []string{p}})
			}
		}
		if len(keys) > 0 {
			body["uniqueKeyPolicy"] = map[string]interface{}{"uniqueKeys": keys}
		}
	}

	payload, _ := json.Marshal(body)
	resp, err := cosmosdb.DoRequest(flow, auth, http.MethodPost, cosmosdb.DBPath(db)+"/colls", "colls", cosmosdb.DBRID(db), headers, payload)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if resp.StatusCode == http.StatusConflict {
		return cosmosdb.ErrorResult(fmt.Sprintf("container %q already exists in database %q", coll, db)), nil
	}
	if err := cosmosdb.CheckResponse(resp); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	obj, err := cosmosdb.DecodeObject(resp)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	// pkPath is authoritative here, and a same-named container earlier in this
	// execution may have cached a different one (delete-then-recreate).
	cosmosdb.SeedPartitionKeyPath(auth, db, coll, pkPath)
	return cosmosdb.ResourceResult(obj, cosmosdb.RequestCharge(resp), fmt.Sprintf("Created container %q in database %q", coll, db)), nil
}

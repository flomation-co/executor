// Package azure_cosmosdb_database_create creates a database in the account.
//
// n8n pins one database into its Cosmos credential and exposes zero database
// operations; here the database is a first-class resource. Provisioned
// throughput at the database level is optional and shared by its containers —
// set at create time only, via headers (manual RU/s or autoscale).
package azure_cosmosdb_database_create

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
	Name         = "Cosmos DB: Create Database"
	Description  = "Create a database in the Cosmos DB account, optionally with shared provisioned throughput (manual RU/s or autoscale)."
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

	{Name: "database", Type: core.ConnectionTypeString, Label: "Database", Placeholder: "The ID of the database to create", Required: true},
	{Name: "throughput", Type: core.ConnectionTypeInteger, Label: "Throughput (RU/s)", Placeholder: "Shared manual throughput, minimum 400 — leave blank for none"},
	{Name: "autoscale_max", Type: core.ConnectionTypeInteger, Label: "Autoscale Max (RU/s)", Placeholder: "Shared autoscale maximum, minimum 1000 — mutually exclusive with Throughput"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Database ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Database"},
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
	headers, err := cosmosdb.ThroughputHeaders(inputs)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}

	body, _ := json.Marshal(map[string]string{"id": db})
	resp, err := cosmosdb.DoRequest(flow, auth, http.MethodPost, "/dbs", "dbs", "", headers, body)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if resp.StatusCode == http.StatusConflict {
		return cosmosdb.ErrorResult(fmt.Sprintf("database %q already exists", db)), nil
	}
	if err := cosmosdb.CheckResponse(resp); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	obj, err := cosmosdb.DecodeObject(resp)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	return cosmosdb.ResourceResult(obj, cosmosdb.RequestCharge(resp), fmt.Sprintf("Created database %q", db)), nil
}

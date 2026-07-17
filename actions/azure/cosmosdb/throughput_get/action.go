// Package azure_cosmosdb_throughput_get reads a container's provisioned
// throughput (its standing "offer"). Offers are account-level resources tied
// to a container by its _rid, so the container is fetched first and the offer
// found with an /offers query — n8n cannot address /offers at all (its signer
// only recognises resource types it can parse out of the URL).
package azure_cosmosdb_throughput_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	cosmosdb "flomation.app/automate/executor/actions/azure/cosmosdb"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cosmos DB: Get Throughput"
	Description  = "Read a container's provisioned throughput — current manual RU/s or autoscale maximum. Containers on shared database throughput or serverless accounts have none."
	Website      = "https://www.flomation.co"
	Icon         = "azure+gauge"
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
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Offer ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Offer"},
	{Name: "request_charge", Type: core.ConnectionTypeString, Label: "Request Charge (RU)"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// SummariseOffer renders the offer's current throughput mode as a sentence.
// Autoscale offers carry content.offerAutopilotSettings.maxThroughput; manual
// offers carry content.offerThroughput.
func SummariseOffer(offer map[string]interface{}) string {
	content, _ := offer["content"].(map[string]interface{})
	if auto, ok := content["offerAutopilotSettings"].(map[string]interface{}); ok {
		if max, ok := auto["maxThroughput"].(float64); ok {
			return fmt.Sprintf("autoscale, max %.0f RU/s", max)
		}
	}
	if manual, ok := content["offerThroughput"].(float64); ok {
		return fmt.Sprintf("manual, %.0f RU/s", manual)
	}
	return "unknown mode"
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

	offer, charge, err := cosmosdb.FindOffer(flow, auth, db, coll)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if offer == nil {
		return cosmosdb.ErrorResult(fmt.Sprintf("container %q has no dedicated throughput offer — it uses shared database throughput, or the account is serverless", coll)), nil
	}
	return cosmosdb.ResourceResult(offer, charge, fmt.Sprintf("Throughput of container %q: %s", coll, SummariseOffer(offer))), nil
}

// Package azure_cosmosdb_throughput_update rescales a container's provisioned
// throughput by rewriting its standing offer — completely absent in n8n, which
// can only set throughput at container create time.
//
// The offer is resolved via the container's _rid, its content mutated (manual
// offerThroughput or autoscale offerAutopilotSettings), and the whole document
// PUT back. Two signing quirks live here: the single-offer PUT signs with the
// offer's _rid LOWERCASED, and switching between manual and autoscale modes is
// a migration the plain REST offer-replace refuses — the server's error is
// surfaced as-is when someone tries.
package azure_cosmosdb_throughput_update

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
	Name         = "Cosmos DB: Update Throughput"
	Description  = "Change a container's provisioned throughput — set a new manual RU/s value or a new autoscale maximum. Switching between manual and autoscale modes must be done in the Azure Portal."
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
	{Name: "throughput", Type: core.ConnectionTypeInteger, Label: "Throughput (RU/s)", Placeholder: "New manual throughput, minimum 400 — for containers already in manual mode"},
	{Name: "autoscale_max", Type: core.ConnectionTypeInteger, Label: "Autoscale Max (RU/s)", Placeholder: "New autoscale maximum, minimum 1000 — for containers already in autoscale mode"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Offer ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Offer"},
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

	manual, hasManual := cosmosdb.OptionalInt("throughput", inputs)
	autoscale, hasAutoscale := cosmosdb.OptionalInt("autoscale_max", inputs)
	switch {
	case hasManual && hasAutoscale:
		return cosmosdb.ErrorResult("throughput and autoscale_max are mutually exclusive — set one or the other"), nil
	case !hasManual && !hasAutoscale:
		return cosmosdb.ErrorResult("set throughput (manual RU/s) or autoscale_max (autoscale maximum RU/s)"), nil
	case hasManual && manual < 400:
		return cosmosdb.ErrorResult("throughput must be at least 400 RU/s"), nil
	case hasAutoscale && autoscale < 1000:
		return cosmosdb.ErrorResult("autoscale_max must be at least 1000 RU/s"), nil
	}

	offer, findCharge, err := cosmosdb.FindOffer(flow, auth, db, coll)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if offer == nil {
		return cosmosdb.ErrorResult(fmt.Sprintf("container %q has no dedicated throughput offer — it uses shared database throughput, or the account is serverless", coll)), nil
	}

	content, _ := offer["content"].(map[string]interface{})
	if content == nil {
		content = map[string]interface{}{}
	}
	summary := ""
	if hasManual {
		content["offerThroughput"] = manual
		summary = fmt.Sprintf("Set throughput of container %q to %d RU/s (manual)", coll, manual)
	} else {
		content["offerAutopilotSettings"] = map[string]interface{}{"maxThroughput": autoscale}
		summary = fmt.Sprintf("Set autoscale maximum of container %q to %d RU/s", coll, autoscale)
	}
	offer["content"] = content

	path, rid, err := cosmosdb.OfferPathAndRID(offer)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	payload, _ := json.Marshal(offer)
	resp, err := cosmosdb.DoRequest(flow, auth, http.MethodPut, path, "offers", rid, nil, payload)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	if err := cosmosdb.CheckResponse(resp); err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	updated, err := cosmosdb.DecodeObject(resp)
	if err != nil {
		return cosmosdb.ErrorResult(err.Error()), nil
	}
	charge := cosmosdb.SumCharges(findCharge, cosmosdb.RequestCharge(resp))
	return cosmosdb.ResourceResult(updated, charge, summary), nil
}

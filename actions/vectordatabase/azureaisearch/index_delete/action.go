package vectordatabase_azureaisearch_index_delete

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azureaisearch "flomation.app/automate/executor/actions/vectordatabase/azureaisearch"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure AI Search: Delete Index"
	Description  = "Delete an Azure AI Search index and every document in it. This cannot be undone. Needs an admin API key."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+trash"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "service_name", Type: core.ConnectionTypeString, Label: "Service Name", Placeholder: "my-search-service — the {name} in https://{name}.search.windows.net"},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://my-search-service.search.windows.net — overrides Service Name (sovereign clouds, private endpoints)"},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Admin key from Settings ▸ Keys — a query key only covers reads", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "2024-07-01 (default)"},
	{Name: "index_name", Type: core.ConnectionTypeString, Label: "Index Name", Placeholder: "products", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Index Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := azureaisearch.GetAuth(inputs)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	name, err := azureaisearch.RequiredString("index_name", inputs)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}

	resp, err := azureaisearch.ExecuteAPI(flow, auth, http.MethodDelete, azureaisearch.IndexPath(name), nil, nil, nil)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	if err := azureaisearch.CheckResponse(auth, resp); err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}

	// A successful delete answers 204 with no body.
	return azureaisearch.ResourceResult(name, map[string]interface{}{"deleted": true, "name": name},
		fmt.Sprintf("Deleted index %q", name)), nil
}

package vectordatabase_azureaisearch_document_count

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azureaisearch "flomation.app/automate/executor/actions/vectordatabase/azureaisearch"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure AI Search: Count Documents"
	Description  = "Count the documents in an index. The count is eventually consistent — a just-finished upload can take a few seconds to show."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+hashtag"
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
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Document Count"},
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

	resp, err := azureaisearch.ExecuteAPI(flow, auth, http.MethodGet,
		azureaisearch.IndexPath(name)+"/docs/$count", nil, nil, nil)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	if err := azureaisearch.CheckResponse(auth, resp); err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}

	// Unlike every other endpoint this one answers text/plain — a bare
	// integer, sometimes with a UTF-8 BOM — not JSON.
	count, err := azureaisearch.ParseCount(resp.Body)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	return map[string]interface{}{
		"count":       count,
		"result":      map[string]interface{}{"index": name, "count": count},
		"tool_result": fmt.Sprintf("Index %q holds %d documents", name, count),
		"success":     true,
		"error":       "",
	}, nil
}

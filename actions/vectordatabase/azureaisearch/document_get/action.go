package vectordatabase_azureaisearch_document_get

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	azureaisearch "flomation.app/automate/executor/actions/vectordatabase/azureaisearch"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure AI Search: Get Document"
	Description  = "Look up a single document in an index by its key. Optionally select specific fields to return."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+file"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "service_name", Type: core.ConnectionTypeString, Label: "Service Name", Placeholder: "my-search-service — the {name} in https://{name}.search.windows.net"},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://my-search-service.search.windows.net — overrides Service Name (sovereign clouds, private endpoints)"},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Admin key from Settings ▸ Keys — a query key only covers reads", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "2024-07-01 (default)"},
	{Name: "index_name", Type: core.ConnectionTypeString, Label: "Index Name", Placeholder: "products", Required: true},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Document Key", Placeholder: "doc-1", Required: true},
	{Name: "select", Type: core.ConnectionTypeString, Label: "Select Fields", Placeholder: "id,content — comma-separated fields to return (blank for all retrievable fields)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Document Key"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Document"},
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
	key, err := azureaisearch.RequiredString("key", inputs)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	if sel := azureaisearch.OptionalString("select", inputs); sel != "" {
		q.Set("$select", sel)
	}
	// The OData lookup path: /docs('{key}'), key quoted-and-escaped.
	path := azureaisearch.IndexPath(name) + "/docs('" + azureaisearch.EscapeDocKey(key) + "')"
	resp, err := azureaisearch.ExecuteAPI(flow, auth, http.MethodGet, path, q, nil, nil)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	if err := azureaisearch.CheckResponse(auth, resp); err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	obj, err := azureaisearch.Decode(resp)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	return azureaisearch.ResourceResult(key, obj, fmt.Sprintf("Retrieved document %q from %q", key, name)), nil
}

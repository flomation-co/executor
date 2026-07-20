package vectordatabase_azureaisearch_index_get_all

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
	Name         = "Azure AI Search: Get Many Indexes"
	Description  = "List every index on the search service. Optionally select specific properties (e.g. name) to keep the output small."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+list"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "service_name", Type: core.ConnectionTypeString, Label: "Service Name", Placeholder: "my-search-service — the {name} in https://{name}.search.windows.net"},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://my-search-service.search.windows.net — overrides Service Name (sovereign clouds, private endpoints)"},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Admin key from Settings ▸ Keys — a query key only covers reads", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "2024-07-01 (default)"},
	{Name: "select", Type: core.ConnectionTypeString, Label: "Select Properties", Placeholder: "name — comma-separated index properties to return (blank for full definitions)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Indexes"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Index Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := azureaisearch.GetAuth(inputs)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}

	// GET /indexes returns every index in one response — the service has no
	// continuation token on this endpoint, so there is nothing to paginate.
	q := url.Values{}
	if sel := azureaisearch.OptionalString("select", inputs); sel != "" {
		q.Set("$select", sel)
	}
	resp, err := azureaisearch.ExecuteAPI(flow, auth, http.MethodGet, "/indexes", q, nil, nil)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	if err := azureaisearch.CheckResponse(auth, resp); err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	items, err := azureaisearch.DecodeValue(resp)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	return azureaisearch.ListResult(items, len(items), fmt.Sprintf("Found %d indexes", len(items))), nil
}

// Package azureaisearch_index_stats implements vectordatabase/azureaisearch/index_stats.
//
// The counts here come from Azure's /stats endpoint, which it recomputes
// periodically rather than on write: seconds after an upload it still reports
// documentCount 0 while /docs/$count already reports the true number
// (verified against a live Free-tier service). That lag is Azure's, not this
// action's — a flow that needs an authoritative count immediately after
// indexing should use document_count.
package vectordatabase_azureaisearch_index_stats

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azureaisearch "flomation.app/automate/executor/actions/vectordatabase/azureaisearch"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure AI Search: Get Index Statistics"
	Description  = "Fetch an index's document count and storage usage (including vector index size) — handy for capacity checks before a bulk upload."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+gauge"
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
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Statistics"},
	{Name: "document_count", Type: core.ConnectionTypeInteger, Label: "Document Count"},
	{Name: "storage_size", Type: core.ConnectionTypeInteger, Label: "Storage Size (bytes)"},
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

	resp, err := azureaisearch.ExecuteAPI(flow, auth, http.MethodGet, azureaisearch.IndexPath(name)+"/stats", nil, nil, nil)
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

	// {"documentCount":N,"storageSize":N,"vectorIndexSize":N} — the numbers
	// decode as float64; surface the two headline figures as typed outputs.
	docCount := asInt(obj["documentCount"])
	storage := asInt(obj["storageSize"])
	out := azureaisearch.ResourceResult(name, obj,
		fmt.Sprintf("Index %q holds %d documents (%d bytes)", name, docCount, storage))
	out["document_count"] = docCount
	out["storage_size"] = storage
	return out, nil
}

func asInt(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}

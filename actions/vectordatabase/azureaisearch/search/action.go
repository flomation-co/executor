package vectordatabase_azureaisearch_search

import (
	"encoding/json"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azureaisearch "flomation.app/automate/executor/actions/vectordatabase/azureaisearch"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure AI Search: Search"
	Description  = "Query an index: keyword (full-text), vector (similarity over an embedding), or hybrid (both fused). Optionally re-rank with a semantic configuration. Results include @search.score."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+magnifying-glass"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "service_name", Type: core.ConnectionTypeString, Label: "Service Name", Placeholder: "my-search-service — the {name} in https://{name}.search.windows.net"},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://my-search-service.search.windows.net — overrides Service Name (sovereign clouds, private endpoints)"},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Admin key from Settings ▸ Keys — a query key only covers reads", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "2024-07-01 (default)"},
	{Name: "index_name", Type: core.ConnectionTypeString, Label: "Index Name", Placeholder: "products", Required: true},
	{Name: "search_text", Type: core.ConnectionTypeString, Label: "Search Text", Placeholder: "What to look for — blank matches everything (*)"},
	{
		Name:  "mode",
		Type:  core.ConnectionTypeString,
		Label: "Search Mode",
		Options: []core.ConnectionOption{
			{Name: "Keyword (full-text)", Value: "keyword"},
			{Name: "Vector (similarity)", Value: "vector"},
			{Name: "Hybrid (keyword + vector)", Value: "hybrid"},
		},
	},
	{Name: "vector", Type: core.ConnectionTypeText, Label: "Query Vector (JSON array)", Placeholder: "[0.12, -0.03, …] — the query embedding; wire in AI ▸ Embed Text", Visible: &core.VisibleWhen{Field: "mode", Values: []string{"vector", "hybrid"}}},
	{Name: "vector_field", Type: core.ConnectionTypeString, Label: "Vector Field", Placeholder: "content_vector (default)", Visible: &core.VisibleWhen{Field: "mode", Values: []string{"vector", "hybrid"}}},
	{Name: "k", Type: core.ConnectionTypeInteger, Label: "Nearest Neighbours (k)", Placeholder: "10 (default) — how many nearest documents the vector query considers", Visible: &core.VisibleWhen{Field: "mode", Values: []string{"vector", "hybrid"}}},
	{Name: "filter", Type: core.ConnectionTypeString, Label: "Filter (OData)", Placeholder: "category eq 'technology' and rating gt 3"},
	{Name: "select", Type: core.ConnectionTypeString, Label: "Select Fields", Placeholder: "id,title,content — comma-separated fields to return"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By", Placeholder: "rating desc — ignored when relevance ranking applies"},
	{Name: "top", Type: core.ConnectionTypeInteger, Label: "Max Results", Placeholder: "50 (default, max 1000)"},
	{Name: "semantic_configuration", Type: core.ConnectionTypeString, Label: "Semantic Configuration", Placeholder: "Name of a semantic configuration on the index — enables semantic re-ranking (Basic tier or higher)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Matches"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Total Match Count"},
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

	mode := azureaisearch.OptionalString("mode", inputs)
	if mode == "" {
		mode = "keyword"
	}
	switch mode {
	case "keyword", "vector", "hybrid":
	default:
		return azureaisearch.ErrorResult(fmt.Sprintf("mode %q is not valid — use keyword, vector, or hybrid", mode)), nil
	}

	// count:true makes the response carry @odata.count — the TOTAL match
	// count, of which "top" results are returned.
	body := map[string]interface{}{"count": true}

	// Keyword and hybrid carry search text ("*" matches everything); a pure
	// vector query carries none — the embedding is the whole query.
	if mode != "vector" {
		st := azureaisearch.OptionalString("search_text", inputs)
		if st == "" {
			st = "*"
		}
		body["search"] = st
	}

	if mode != "keyword" {
		rawVec := azureaisearch.OptionalString("vector", inputs)
		if rawVec == "" {
			return azureaisearch.ErrorResult(fmt.Sprintf("vector is required for a %s search — wire in a query embedding as a JSON array", mode)), nil
		}
		var vec []float64
		if err := json.Unmarshal([]byte(rawVec), &vec); err != nil || len(vec) == 0 {
			return azureaisearch.ErrorResult("vector must be a non-empty JSON array of numbers, e.g. [0.12, -0.03]"), nil
		}
		field := azureaisearch.OptionalString("vector_field", inputs)
		if field == "" {
			field = "content_vector"
		}
		k, set := azureaisearch.OptionalInt("k", inputs)
		if !set || k <= 0 {
			k = 10
		}
		body["vectorQueries"] = []interface{}{
			map[string]interface{}{"kind": "vector", "vector": vec, "fields": field, "k": k},
		}
	}

	azureaisearch.SetIfPresent(body, inputs, "filter", "filter")
	azureaisearch.SetIfPresent(body, inputs, "select", "select")
	azureaisearch.SetIfPresent(body, inputs, "orderby", "order_by")
	top, set := azureaisearch.OptionalInt("top", inputs)
	body["top"] = azureaisearch.ClampLimit(top, set)

	// A named semantic configuration switches the query to the semantic
	// ranker (L2 re-ranking; requires Basic tier or higher on the service).
	if sc := azureaisearch.OptionalString("semantic_configuration", inputs); sc != "" {
		body["queryType"] = "semantic"
		body["semanticConfiguration"] = sc
	}

	resp, err := azureaisearch.ExecuteAPI(flow, auth, http.MethodPost,
		azureaisearch.IndexPath(name)+"/docs/search", nil, body, nil)
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
	items, _ := obj["value"].([]interface{})
	if items == nil {
		items = []interface{}{}
	}
	total := len(items)
	if c, ok := obj["@odata.count"].(float64); ok {
		total = int(c)
	}

	summary := fmt.Sprintf("Found %d matching documents (%s search", total, mode)
	if _, semantic := body["semanticConfiguration"]; semantic {
		summary += ", semantic ranking"
	}
	summary += ")"
	if len(items) < total {
		summary += fmt.Sprintf(", returning the top %d", len(items))
	}
	return azureaisearch.ListResult(items, total, summary), nil
}

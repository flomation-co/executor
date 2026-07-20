package vectordatabase_azureaisearch_document_delete

import (
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	azureaisearch "flomation.app/automate/executor/actions/vectordatabase/azureaisearch"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure AI Search: Delete Documents"
	Description  = "Delete documents from an index by key. Deleting a key that does not exist counts as success (the service treats it as already gone). Needs an admin API key."
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
	{Name: "key_field", Type: core.ConnectionTypeString, Label: "Key Field", Placeholder: "id — the index's key field name", Required: true},
	{Name: "keys", Type: core.ConnectionTypeString, Label: "Document Keys", Placeholder: "doc-1, doc-2 — comma-separated keys to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Per-Document Status"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Documents Deleted"},
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
	keyField, err := azureaisearch.RequiredString("key_field", inputs)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	rawKeys, err := azureaisearch.RequiredString("keys", inputs)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}

	// Deletes go through the same batch endpoint as uploads: one
	// {"@search.action":"delete"} entry per key, carrying only the key field.
	value := []interface{}{}
	for _, k := range strings.Split(rawKeys, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		value = append(value, map[string]interface{}{"@search.action": "delete", keyField: k})
	}
	if len(value) == 0 {
		return azureaisearch.ErrorResult("keys is required — provide at least one document key"), nil
	}

	resp, err := azureaisearch.ExecuteAPI(flow, auth, http.MethodPost,
		azureaisearch.IndexPath(name)+"/docs/index", nil, map[string]interface{}{"value": value}, nil)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	if err := azureaisearch.CheckResponse(auth, resp); err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}

	statuses, failed, err := azureaisearch.ParseIndexingStatuses(resp)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	deleted := len(statuses) - len(failed)
	if len(failed) > 0 {
		out := azureaisearch.ErrorResult(fmt.Sprintf("%d of %d deletes failed: %s",
			len(failed), len(statuses), azureaisearch.SummariseFailedKeys(failed)))
		out["results"] = statuses
		out["count"] = deleted
		return out, nil
	}
	return azureaisearch.ListResult(statuses, deleted,
		fmt.Sprintf("Deleted %d documents from %q", deleted, name)), nil
}

package vectordatabase_azureaisearch_document_upload

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azureaisearch "flomation.app/automate/executor/actions/vectordatabase/azureaisearch"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure AI Search: Upload Documents"
	Description  = "Add or update documents in an index. Merge or Upload (the default) upserts; Merge updates existing documents only; Upload replaces whole documents. Needs an admin API key."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+arrow-up"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "service_name", Type: core.ConnectionTypeString, Label: "Service Name", Placeholder: "my-search-service — the {name} in https://{name}.search.windows.net"},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://my-search-service.search.windows.net — overrides Service Name (sovereign clouds, private endpoints)"},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Admin key from Settings ▸ Keys — a query key only covers reads", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "2024-07-01 (default)"},
	{Name: "index_name", Type: core.ConnectionTypeString, Label: "Index Name", Placeholder: "products", Required: true},
	{Name: "documents", Type: core.ConnectionTypeObject, Label: "Documents (JSON array)", Placeholder: `[{"id":"1","content":"…","content_vector":[0.1,0.2]}] — each must carry the index's key field`, Required: true},
	{
		Name:  "action",
		Type:  core.ConnectionTypeString,
		Label: "Write Behaviour",
		Options: []core.ConnectionOption{
			{Name: "Merge or Upload (upsert)", Value: "mergeOrUpload"},
			{Name: "Upload (replace whole document)", Value: "upload"},
			{Name: "Merge (update existing only)", Value: "merge"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Per-Document Status"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Documents Accepted"},
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

	raw, err := azureaisearch.OptionalJSON("documents", inputs)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	var docs []interface{}
	switch v := raw.(type) {
	case []interface{}:
		docs = v
	case map[string]interface{}:
		// A single document is a natural thing to wire in — accept it bare.
		docs = []interface{}{v}
	default:
		return azureaisearch.ErrorResult(`documents must be a JSON array of objects, e.g. [{"id":"1","content":"…"}]`), nil
	}
	if len(docs) == 0 {
		return azureaisearch.ErrorResult("documents is required — provide at least one document"), nil
	}

	action := azureaisearch.OptionalString("action", inputs)
	if action == "" {
		action = "mergeOrUpload"
	}
	switch action {
	case "upload", "merge", "mergeOrUpload":
	default:
		return azureaisearch.ErrorResult(fmt.Sprintf("action %q is not valid — use upload, merge, or mergeOrUpload", action)), nil
	}

	// Each document is stamped with its write behaviour: the batch body is
	// {"value":[{"@search.action":…, …doc}, …]}.
	value := make([]interface{}, 0, len(docs))
	for i, d := range docs {
		obj, ok := d.(map[string]interface{})
		if !ok {
			return azureaisearch.ErrorResult(fmt.Sprintf("documents[%d] is not a JSON object", i)), nil
		}
		wrapped := make(map[string]interface{}, len(obj)+1)
		for k, v := range obj {
			wrapped[k] = v
		}
		wrapped["@search.action"] = action
		value = append(value, wrapped)
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
	accepted := len(statuses) - len(failed)
	if len(failed) > 0 {
		out := azureaisearch.ErrorResult(fmt.Sprintf("%d of %d documents failed: %s",
			len(failed), len(statuses), azureaisearch.SummariseFailedKeys(failed)))
		out["results"] = statuses
		out["count"] = accepted
		return out, nil
	}
	return azureaisearch.ListResult(statuses, accepted,
		fmt.Sprintf("Indexed %d documents into %q (%s)", accepted, name, action)), nil
}

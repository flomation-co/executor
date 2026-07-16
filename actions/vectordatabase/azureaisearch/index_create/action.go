package vectordatabase_azureaisearch_index_create

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azureaisearch "flomation.app/automate/executor/actions/vectordatabase/azureaisearch"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure AI Search: Create or Update Index"
	Description  = "Create an Azure AI Search index from a full index definition (fields, vector search profiles, semantic configuration), or update it in place. Needs an admin API key."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+plus"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "service_name", Type: core.ConnectionTypeString, Label: "Service Name", Placeholder: "my-search-service — the {name} in https://{name}.search.windows.net"},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://my-search-service.search.windows.net — overrides Service Name (sovereign clouds, private endpoints)"},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Admin key from Settings ▸ Keys — a query key only covers reads", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "2024-07-01 (default)"},
	{Name: "index_name", Type: core.ConnectionTypeString, Label: "Index Name", Placeholder: "products — lowercase letters, digits and dashes", Required: true},
	{Name: "definition", Type: core.ConnectionTypeObject, Label: "Index Definition (JSON)", Placeholder: `{"fields":[{"name":"id","type":"Edm.String","key":true},{"name":"content","type":"Edm.String","searchable":true},{"name":"content_vector","type":"Collection(Edm.Single)","dimensions":1536,"vectorSearchProfile":"default"}],"vectorSearch":{...}}`, Required: true},
	{Name: "only_if_missing", Type: core.ConnectionTypeBoolean, Label: "Only Create If Missing", Placeholder: "Fail softly instead of overwriting when the index already exists"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Index Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Index"},
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
	raw, err := azureaisearch.OptionalJSON("definition", inputs)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	def, ok := raw.(map[string]interface{})
	if !ok || len(def) == 0 {
		return azureaisearch.ErrorResult(`definition must be a JSON object — the full index definition, e.g. {"fields":[...]}`), nil
	}
	// The URL names the index; the body must agree, so the input wins over
	// whatever "name" the pasted definition carries.
	def["name"] = name

	// PUT /indexes/{name} is create-or-update. If-None-Match: * makes it
	// create-only (an existing index answers 412). Prefer asks the service to
	// return the stored definition on updates, which otherwise answer 204.
	extra := http.Header{"Prefer": []string{"return=representation"}}
	onlyIfMissing := azureaisearch.OptionalBool("only_if_missing", inputs)
	if onlyIfMissing {
		extra.Set("If-None-Match", "*")
	}

	resp, err := azureaisearch.ExecuteAPI(flow, auth, http.MethodPut, azureaisearch.IndexPath(name), nil, def, extra)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	if onlyIfMissing && resp.StatusCode == http.StatusPreconditionFailed {
		return azureaisearch.ErrorResult(fmt.Sprintf("index %q already exists (Only Create If Missing is on)", name)), nil
	}
	if err := azureaisearch.CheckResponse(auth, resp); err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}

	obj, err := azureaisearch.Decode(resp)
	if err != nil {
		return azureaisearch.ErrorResult(err.Error()), nil
	}
	verb := "Updated"
	if resp.StatusCode == http.StatusCreated {
		verb = "Created"
	}
	if len(obj) == 0 {
		// A 204 update without representation — echo what was submitted.
		obj = def
	}
	return azureaisearch.ResourceResult(name, obj, fmt.Sprintf("%s index %q", verb, name)), nil
}

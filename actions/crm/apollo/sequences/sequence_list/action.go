package sequence_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Sequences: List"
	Description  = "List Apollo sequences (emailer campaigns), optionally filtered by name."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+list"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "q_name", Type: core.ConnectionTypeString, Label: "Name Contains", Placeholder: "Filter sequences by name"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1"},
	{Name: "per_page", Type: core.ConnectionTypeInteger, Label: "Per Page", Placeholder: "25 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Sequences"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	apollo_common.SetString(body, "q_name", "q_name", inputs)
	apollo_common.SetInt(body, "page", "page", inputs)
	apollo_common.SetInt(body, "per_page", "per_page", inputs)

	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/emailer_campaigns/search", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	sequences := apollo_common.Arr(resp, "emailer_campaigns")
	return apollo_common.ListResult(sequences, fmt.Sprintf("Found %d sequences", len(sequences))), nil
}

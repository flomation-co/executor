package people_bulk_match

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Person: Bulk Enrich"
	Description  = "Enrich up to 10 people at once from a JSON array of match criteria."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+bolt"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "details", Type: core.ConnectionTypeText, Label: "Details (JSON array)", Placeholder: `[{"email":"a@x.com"},{"first_name":"Ada","last_name":"Lovelace","domain":"x.com"}]`, Required: true},
	{Name: "reveal_personal_emails", Type: core.ConnectionTypeBoolean, Label: "Reveal Personal Emails", Placeholder: "Consumes credits"},
	{Name: "reveal_phone_number", Type: core.ConnectionTypeBoolean, Label: "Reveal Phone Number", Placeholder: "Consumes credits"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Matches"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	raw := apollo_common.OptionalString("details", inputs)
	if raw == "" {
		return apollo_common.ErrorResult("details (a JSON array of people) is required"), nil
	}
	arr, err := apollo_common.ParseJSONArray("details", inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"details": arr}
	apollo_common.SetBool(body, "reveal_personal_emails", "reveal_personal_emails", inputs)
	apollo_common.SetBool(body, "reveal_phone_number", "reveal_phone_number", inputs)

	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/people/bulk_match", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	matches := apollo_common.Arr(resp, "matches")
	return apollo_common.ListResult(matches, fmt.Sprintf("Enriched %d people", len(matches))), nil
}

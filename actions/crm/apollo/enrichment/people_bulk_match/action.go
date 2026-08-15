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
	{Name: "reveal_personal_emails", Type: core.ConnectionTypeBoolean, Label: "Reveal Personal Emails", Placeholder: "ON by default for this enrichment action — 1 credit per email, charged only if found. The box renders unticked until set; tick then untick it to explicitly turn reveal off.", Value: true},
	{Name: "reveal_phone_number", Type: core.ConnectionTypeBoolean, Label: "Reveal Phone Number", Placeholder: "8 credits; requires a Webhook URL (delivered asynchronously)"},
	{Name: "webhook_url", Type: core.ConnectionTypeString, Label: "Webhook URL (required for phone reveal)", Placeholder: "https://… public HTTPS endpoint", Visible: &core.VisibleWhen{Field: "reveal_phone_number", Values: []string{"true"}}},
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
	// Email reveal defaults ON — see people_match for the reasoning. Note this
	// action is a BULK call, so the cost scales with the number of details
	// supplied: 1 credit per email actually found.
	revealEmails := apollo_common.BoolValueDefault("reveal_personal_emails", inputs, true)
	body["reveal_personal_emails"] = revealEmails
	apollo_common.SetBool(body, "reveal_phone_number", "reveal_phone_number", inputs)
	apollo_common.SetString(body, "webhook_url", "webhook_url", inputs)

	// Phone numbers are delivered asynchronously to a webhook, so Apollo requires
	// webhook_url alongside reveal_phone_number.
	if apollo_common.BoolValue("reveal_phone_number", inputs) && apollo_common.OptionalString("webhook_url", inputs) == "" {
		return apollo_common.ErrorResult("Reveal Phone Number requires a Webhook URL: Apollo returns phone numbers asynchronously and delivers them to that URL, not in this response. Provide a public HTTPS Webhook URL, or turn off Reveal Phone Number (email reveal does not need one)."), nil
	}

	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/people/bulk_match", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	matches := apollo_common.Arr(resp, "matches")
	summary := apollo_common.GatePrefix(fmt.Sprintf("Enriched %d people", len(matches)), matches)
	return apollo_common.ListResult(matches, summary), nil
}

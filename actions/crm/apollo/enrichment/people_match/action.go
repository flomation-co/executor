package people_match

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Person: Enrich"
	Description  = "Enrich a single person in Apollo by email, name or LinkedIn URL."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+bolt"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "ada@example.com"},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Ada"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Lovelace"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Full Name", Placeholder: "Ada Lovelace"},
	{Name: "organization_name", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Analytical Engines Ltd"},
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Company Domain", Placeholder: "example.com"},
	{Name: "linkedin_url", Type: core.ConnectionTypeString, Label: "LinkedIn URL", Placeholder: "https://www.linkedin.com/in/…"},
	{Name: "reveal_personal_emails", Type: core.ConnectionTypeBoolean, Label: "Reveal Personal Emails", Placeholder: "ON by default for this enrichment action — 1 credit per email, charged only if found. The box renders unticked until set; tick then untick it to explicitly turn reveal off.", Value: true},
	{Name: "reveal_phone_number", Type: core.ConnectionTypeBoolean, Label: "Reveal Phone Number", Placeholder: "8 credits; requires a Webhook URL (delivered asynchronously)"},
	{Name: "webhook_url", Type: core.ConnectionTypeString, Label: "Webhook URL (required for phone reveal)", Placeholder: "https://… public HTTPS endpoint", Visible: &core.VisibleWhen{Field: "reveal_phone_number", Values: []string{"true"}}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Person ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Person"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	apollo_common.SetString(body, "email", "email", inputs)
	apollo_common.SetString(body, "first_name", "first_name", inputs)
	apollo_common.SetString(body, "last_name", "last_name", inputs)
	apollo_common.SetString(body, "name", "name", inputs)
	apollo_common.SetString(body, "organization_name", "organization_name", inputs)
	apollo_common.SetString(body, "domain", "domain", inputs)
	apollo_common.SetString(body, "linkedin_url", "linkedin_url", inputs)
	// Email reveal defaults ON. This is an enrichment action — the author called
	// it to obtain contact data, and Apollo returns a null email unless asked,
	// so defaulting off made the obvious call quietly useless. The cost is
	// bounded and predictable: 1 credit, and only when an email is actually
	// found. An author who wants a no-cost lookup can switch it off, and that
	// choice is honoured because an untouched input stays distinguishable from
	// an explicit false.
	revealEmails := apollo_common.BoolValueDefault("reveal_personal_emails", inputs, true)
	body["reveal_personal_emails"] = revealEmails
	apollo_common.SetBool(body, "reveal_phone_number", "reveal_phone_number", inputs)
	apollo_common.SetString(body, "webhook_url", "webhook_url", inputs)

	// Apollo delivers phone results ASYNCHRONOUSLY to a webhook, so it requires
	// webhook_url whenever reveal_phone_number is set. Without it the call is
	// rejected (or the numbers simply never arrive), which is a confusing way to
	// fail — catch it here with an explanation instead.
	if apollo_common.BoolValue("reveal_phone_number", inputs) && apollo_common.OptionalString("webhook_url", inputs) == "" {
		return apollo_common.ErrorResult("Reveal Phone Number requires a Webhook URL: Apollo returns phone numbers asynchronously and delivers them to that URL, not in this response. Provide a public HTTPS Webhook URL, or turn off Reveal Phone Number (email reveal does not need one)."), nil
	}

	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/people/match", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	person := apollo_common.Obj(resp, "person")
	if person == nil {
		return apollo_common.ErrorResult("no matching person found"), nil
	}
	name, _ := person["name"].(string)
	// If the record came back with personal data withheld, say so — and say why,
	// since the usual cause is the un-set reveal flag rather than the plan.
	summary := apollo_common.GatePrefix(fmt.Sprintf("Enriched %s", name), []map[string]interface{}{person})
	if hint := apollo_common.RevealHint(person, revealEmails); hint != "" {
		summary += "\n" + hint
	}
	return apollo_common.ObjectResult("", person, summary), nil
}

package contact_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Contact: Create"
	Description  = "Create a contact in your Apollo CRM. Returns the contact ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+plus"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Ada"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Lovelace"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Job Title", Placeholder: "Head of Engineering"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "ada@example.com"},
	{Name: "organization_name", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Analytical Engines Ltd"},
	{Name: "website_url", Type: core.ConnectionTypeString, Label: "Company Website", Placeholder: "https://example.com"},
	{Name: "present_raw_address", Type: core.ConnectionTypeString, Label: "Address", Placeholder: "London, United Kingdom"},
	{Name: "direct_phone", Type: core.ConnectionTypeString, Label: "Direct Phone", Placeholder: "+44 20 7946 0000"},
	{Name: "mobile_phone", Type: core.ConnectionTypeString, Label: "Mobile Phone", Placeholder: "+44 7700 900000"},
	{Name: "label_names", Type: core.ConnectionTypeString, Label: "List Names", Placeholder: "Prospects, Q3 Outreach (comma-separated)"},
	{Name: "run_dedupe", Type: core.ConnectionTypeBoolean, Label: "Run Deduplication", Placeholder: "Check for an existing matching contact first"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON)", Placeholder: `{"typed_custom_fields":{"…":"…"}}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Contact"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	apollo_common.SetString(body, "first_name", "first_name", inputs)
	apollo_common.SetString(body, "last_name", "last_name", inputs)
	apollo_common.SetString(body, "title", "title", inputs)
	apollo_common.SetString(body, "email", "email", inputs)
	apollo_common.SetString(body, "organization_name", "organization_name", inputs)
	apollo_common.SetString(body, "website_url", "website_url", inputs)
	apollo_common.SetString(body, "present_raw_address", "present_raw_address", inputs)
	apollo_common.SetString(body, "direct_phone", "direct_phone", inputs)
	apollo_common.SetString(body, "mobile_phone", "mobile_phone", inputs)
	apollo_common.SetList(body, "label_names", "label_names", inputs)
	apollo_common.SetBool(body, "run_dedupe", "run_dedupe", inputs)

	extra, err := apollo_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	apollo_common.MergeFields(body, extra)

	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/contacts", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	contact := apollo_common.Obj(resp, "contact")
	if contact == nil {
		return apollo_common.ErrorResult("contact was not created"), nil
	}
	return apollo_common.ObjectResult("", contact, fmt.Sprintf("Created contact %s", apollo_common.IDOf(contact))), nil
}

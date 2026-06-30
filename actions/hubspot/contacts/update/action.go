package hubspot_contacts_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	hubspot "flomation.app/automate/executor/actions/hubspot"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update Contact"
	Description  = "Update properties on an existing HubSpot contact. Only the fields you set are changed."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot+pencil"
	Date         = "30/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HubSpot Private App Token", Placeholder: "pat-...", Required: true},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "12345", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "jane@example.com"},
	{Name: "firstname", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Jane"},
	{Name: "lastname", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Doe"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "+1 555 0100"},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company", Placeholder: "Acme Inc"},
	{Name: "jobtitle", Type: core.ConnectionTypeString, Label: "Job Title", Placeholder: "Head of Sales"},
	{Name: "website", Type: core.ConnectionTypeString, Label: "Website", Placeholder: "https://example.com"},
	{Name: "additional_properties", Type: core.ConnectionTypeKeyValueArray, Label: "Additional Properties", Placeholder: "Any other contact property (key = HubSpot internal name)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
	{Name: "properties", Type: core.ConnectionTypeObject, Label: "Properties"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Contact"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := hubspot.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	id, err := hubspot.RequiredString("contact_id", inputs)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	props := hubspot.BuildProperties(inputs, "email", "firstname", "lastname", "phone", "company", "jobtitle", "website")
	if len(props) == 0 {
		return hubspot.ErrorResult("at least one property to update is required"), nil
	}

	obj, err := hubspot.UpdateObject(apiKey, "contacts", id, props)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	return hubspot.ObjectResult(obj, fmt.Sprintf("Updated contact %s", id)), nil
}

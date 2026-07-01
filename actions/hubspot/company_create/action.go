package hubspot_company_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	hubspot "flomation.app/automate/executor/actions/hubspot"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Company: Create"
	Description  = "Create a new company in HubSpot. Set common fields directly or add any other property via Additional Properties. Returns the company ID."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot+plus"
	Date         = "30/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HubSpot Private App Token", Placeholder: "pat-...", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Acme Inc"},
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Domain", Placeholder: "acme.com"},
	{Name: "industry", Type: core.ConnectionTypeString, Label: "Industry", Placeholder: "COMPUTER_SOFTWARE"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "+1 555 0100"},
	{Name: "city", Type: core.ConnectionTypeString, Label: "City", Placeholder: "London"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State/Region", Placeholder: "England"},
	{Name: "country", Type: core.ConnectionTypeString, Label: "Country", Placeholder: "United Kingdom"},
	{Name: "website", Type: core.ConnectionTypeString, Label: "Website", Placeholder: "https://acme.com"},
	{Name: "additional_properties", Type: core.ConnectionTypeKeyValueArray, Label: "Additional Properties", Placeholder: "Any other company property (key = HubSpot internal name)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Company ID"},
	{Name: "properties", Type: core.ConnectionTypeObject, Label: "Properties"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Company"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := hubspot.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	props := hubspot.BuildProperties(inputs, "name", "domain", "industry", "phone", "city", "state", "country", "website")
	if len(props) == 0 {
		return hubspot.ErrorResult("at least one company property is required"), nil
	}

	obj, err := hubspot.CreateObject(apiKey, "companies", props, nil)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	id, _ := obj["id"].(string)
	return hubspot.ObjectResult(obj, fmt.Sprintf("Created company %s", id)), nil
}

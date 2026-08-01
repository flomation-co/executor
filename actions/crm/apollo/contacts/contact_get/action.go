package contact_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Contact: Get by ID"
	Description  = "Fetch a single Apollo contact by its ID (Apollo master key required)."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+eye"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey} (master key)", Required: true},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Apollo contact ID", Required: true},
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
	id, err := apollo_common.RequiredString("contact_id", inputs)
	if err != nil {
		return apollo_common.ErrorResult("a contact ID is required"), nil
	}

	resp, err := apollo_common.NewClient(apiKey).Get(flow, "/contacts/"+id, nil)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	contact := apollo_common.Obj(resp, "contact")
	if contact == nil {
		return apollo_common.ErrorResult(fmt.Sprintf("no contact found for ID %s", id)), nil
	}
	return apollo_common.ObjectResult(id, contact, fmt.Sprintf("Contact %s", id)), nil
}

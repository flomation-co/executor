package hubspot_associate

import (
	"fmt"

	core "flomation.app/automate/executor"
	hubspot "flomation.app/automate/executor/actions/hubspot"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Association: Link Records"
	Description  = "Link two HubSpot records with the default association, e.g. attach a contact to a company or a deal. Direction is from -> to."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot+link"
	Date         = "30/06/2026"
	Type         = core.ActionTypeAction
)

var objectTypeOptions = []core.ConnectionOption{
	{Name: "Contact", Value: "contacts"},
	{Name: "Company", Value: "companies"},
	{Name: "Deal", Value: "deals"},
	{Name: "Ticket", Value: "tickets"},
}

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HubSpot Private App Token", Placeholder: "pat-...", Required: true},
	{Name: "from_object_type", Type: core.ConnectionTypeString, Label: "From Object Type", Placeholder: "contacts", Required: true, Options: objectTypeOptions},
	{Name: "from_id", Type: core.ConnectionTypeString, Label: "From Record ID", Placeholder: "12345", Required: true},
	{Name: "to_object_type", Type: core.ConnectionTypeString, Label: "To Object Type", Placeholder: "companies", Required: true, Options: objectTypeOptions},
	{Name: "to_id", Type: core.ConnectionTypeString, Label: "To Record ID", Placeholder: "67890", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Association"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := hubspot.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	fromType, err := hubspot.RequiredString("from_object_type", inputs)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}
	fromID, err := hubspot.RequiredString("from_id", inputs)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}
	toType, err := hubspot.RequiredString("to_object_type", inputs)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}
	toID, err := hubspot.RequiredString("to_id", inputs)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	obj, err := hubspot.AssociateDefault(apiKey, fromType, fromID, toType, toID)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	return map[string]interface{}{
		"result":      obj,
		"tool_result": fmt.Sprintf("Associated %s %s with %s %s", fromType, fromID, toType, toID),
		"success":     true,
		"error":       "",
	}, nil
}

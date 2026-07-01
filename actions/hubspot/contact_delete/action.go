package hubspot_contact_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	hubspot "flomation.app/automate/executor/actions/hubspot"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Contact: Delete"
	Description  = "Archive (soft-delete) a HubSpot contact by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot+trash"
	Date         = "30/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HubSpot Private App Token", Placeholder: "pat-...", Required: true},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "12345", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
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

	if err := hubspot.ArchiveObject(apiKey, "contacts", id); err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	return map[string]interface{}{
		"id":          id,
		"tool_result": fmt.Sprintf("Archived contact %s", id),
		"success":     true,
		"error":       "",
	}, nil
}

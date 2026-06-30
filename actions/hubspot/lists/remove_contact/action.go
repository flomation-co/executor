package hubspot_lists_remove_contact

import (
	"fmt"

	core "flomation.app/automate/executor"
	hubspot "flomation.app/automate/executor/actions/hubspot"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Remove Contacts from List"
	Description  = "Remove contacts from a HubSpot static list by contact ID. Only works on static (not dynamic) lists."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot+list"
	Date         = "30/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HubSpot Private App Token", Placeholder: "pat-...", Required: true},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "List ID", Placeholder: "Static list ID", Required: true},
	{Name: "contact_ids", Type: core.ConnectionTypeString, Label: "Contact IDs", Placeholder: "Comma-separated contact IDs (vids)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "updated", Type: core.ConnectionTypeObject, Label: "Updated Contact IDs"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := hubspot.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	listID, err := hubspot.RequiredString("list_id", inputs)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	vids := hubspot.ToInterfaceList(hubspot.CSVToList(hubspot.OptionalString("contact_ids", inputs)))
	if len(vids) == 0 {
		return hubspot.ErrorResult("at least one contact ID is required"), nil
	}

	resp, err := hubspot.ListMembership(apiKey, listID, "remove", vids, nil)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	return map[string]interface{}{
		"result":      resp,
		"updated":     resp["updated"],
		"tool_result": fmt.Sprintf("Removed contacts from list %s", listID),
		"success":     true,
		"error":       "",
	}, nil
}

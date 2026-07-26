package crm_salesforce_event_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Event"
	Description  = "Take a single appointment out of the Salesforce diary. It goes to the Recycle Bin rather than disappearing, so it can still be restored for the next 15 days."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID", Placeholder: "00U5f00000AbCdEEAV", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Event ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Event"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	eventID := salesforce.OptionalString("event_id", inputs)
	if err := salesforce.ValidateRecordID(eventID); err != nil {
		return nil, err
	}

	// This removes one appointment. Deleting an occurrence of a repeating series
	// this way is what Salesforce calls a single-occurrence delete; use Cancel
	// Event Series to clear the rest of the series along with it.
	if err := salesforce.DeleteRecord(instanceURL, token, "Event", eventID); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// A delete answers 204 No Content, so the ID we were given is the only thing
	// there is to hand on — and it is what a later step needs to report on.
	return salesforce.RecordResult(eventID, nil, fmt.Sprintf("Sent event %s to the Recycle Bin", eventID)), nil
}

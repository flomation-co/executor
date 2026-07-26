package crm_salesforce_event_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Event"
	Description  = "Look up one calendar event in Salesforce by its record ID and return everything on it — when it is, where it is, who it is with."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+eye"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID", Placeholder: "00U5f00000AbCdEEAV", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Subject, StartDateTime, Location — leave blank for every field"},
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

	// Check the field list here as well as inside the shared client, so a typo
	// is reported as the configuration mistake it is rather than landing on the
	// error port as though Salesforce had refused the call.
	fields := salesforce.OptionalString("fields", inputs)
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, err
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Event", eventID, fields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Fetched event %s", eventID)
	if subject, ok := record["Subject"].(string); ok && subject != "" {
		summary = fmt.Sprintf("Fetched event %q (%s)", subject, eventID)
	}
	return salesforce.RecordResult(eventID, record, summary), nil
}

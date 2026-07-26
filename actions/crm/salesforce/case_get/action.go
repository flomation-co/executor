// Package crm_salesforce_case_get reads one support Case by its record ID.
//
// This is the "look it up" half of every helpdesk flow: a trigger hands over a
// case ID and the next node needs the subject, status, priority and owner to
// decide what to do with it. Field selection is offered as a parity-plus — n8n
// has no way to narrow this read, so every lookup drags the whole record back
// even when the flow only wanted the status.
package crm_salesforce_case_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Case"
	Description  = "Look up a single Salesforce case by its record ID and return its details. Leave Fields blank to get everything the connected user can see."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	{Name: "case_id", Type: core.ConnectionTypeString, Label: "Case ID", Placeholder: "5005f00000XyzAAAAQ — the case record ID", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Leave blank for every field, or list them: CaseNumber,Subject,Status,Priority"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Case ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Case"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	caseID, err := salesforce.RequiredString("case_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("case_id is required — the ID of the case to read")
	}
	// Validate the ID and the field list up front rather than letting GetRecord
	// do it inside the call. Both are configuration mistakes and must fail hard;
	// once past this point every error is Salesforce's answer and belongs on the
	// error port as data.
	if err := salesforce.ValidateRecordID(caseID); err != nil {
		return nil, err
	}
	fields := salesforce.OptionalString("fields", inputs)
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, err
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Case", caseID, fields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	return salesforce.RecordResult(caseID, record, summarise(caseID, record)), nil
}

// summarise describes what came back, leading with the case number because that
// is the reference a customer quotes, not the 18-character record ID. Both are
// optional here: a narrowed field list may have excluded either of them.
func summarise(caseID string, record map[string]interface{}) string {
	number, _ := record["CaseNumber"].(string)
	subject, _ := record["Subject"].(string)
	switch {
	case number != "" && subject != "":
		return fmt.Sprintf("Retrieved case %s — %s", number, subject)
	case number != "":
		return fmt.Sprintf("Retrieved case %s", number)
	case subject != "":
		return fmt.Sprintf("Retrieved case %s — %s", caseID, subject)
	default:
		return fmt.Sprintf("Retrieved case %s", caseID)
	}
}

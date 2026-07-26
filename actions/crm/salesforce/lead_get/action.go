// Package crm_salesforce_lead_get retrieves a single Lead by its record ID.
//
// The plain read is deliberately the whole record: what a flow needs from a lead
// is rarely known when the action is dropped on the canvas, and a partial record
// produces the "why is that field empty?" support ticket. A field list is
// offered for the cases where the lead is large and only one value is wanted.
package crm_salesforce_lead_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Lead"
	Description  = "Look up one lead by its Salesforce record ID and return everything the connected user is allowed to see."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "lead_id", Type: core.ConnectionTypeString, Label: "Lead ID", Placeholder: "00Q5f000004XyzAEAS", Required: true},
	// Leaving this blank returns every readable field, which is what almost
	// everyone wants. n8n has no equivalent — its lead read is all-or-nothing.
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields to Return", Placeholder: "Leave blank for all fields, or e.g. Id,FirstName,LastName,Email"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Lead ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Lead"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	leadID, err := salesforce.RequiredString("lead_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("lead_id is required — the Salesforce record ID of the lead, e.g. 00Q5f000004XyzAEAS")
	}
	// A malformed ID is a configuration mistake, so catch it locally: the
	// server-side MALFORMED_ID error does not say which input was wrong.
	if err := salesforce.ValidateRecordID(leadID); err != nil {
		return nil, err
	}

	// A misspelled field name is rejected locally and never reaches Salesforce,
	// so it is a configuration mistake and takes the hard error return — the same
	// contract account_get and case_get follow. Left to GetRecord it comes back
	// on the soft error port, where it reads as though Salesforce had refused the
	// request and the flow keeps running down its failure branch, retrying a typo
	// no retry can fix.
	fields := salesforce.OptionalString("fields", inputs)
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, fmt.Errorf("Fields — %w", err)
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Lead", leadID, fields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	return salesforce.RecordResult(leadID, record, fmt.Sprintf("Retrieved lead %s", describeLead(record, leadID))), nil
}

// describeLead builds a human label for the summary line. A record ID means
// nothing to the person reading the run history, so lead "Jane Smith (Acme
// Ltd)" beats lead "00Q5f000004XyzAEAS" — but the ID is the honest fallback
// when a narrow field list left the name out.
func describeLead(record map[string]interface{}, leadID string) string {
	name := joinNonEmpty(text(record["FirstName"]), text(record["LastName"]))
	company := text(record["Company"])
	switch {
	case name != "" && company != "":
		return name + " at " + company
	case name != "":
		return name
	case company != "":
		return company
	default:
		return leadID
	}
}

func joinNonEmpty(first, last string) string {
	switch {
	case first != "" && last != "":
		return first + " " + last
	case first != "":
		return first
	default:
		return last
	}
}

// text renders a field value as a string, tolerating the JSON nulls Salesforce
// returns for every unset field on a full-record read.
func text(v interface{}) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

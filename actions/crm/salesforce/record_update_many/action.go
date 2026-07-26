// Package crm_salesforce_record_update_many updates many records of one object
// in a single Salesforce API call.
//
// Same allowance argument as Create Many — 200 records per request instead of
// one — with one extra rule the operator has to get right: every record in the
// list must carry its own Id, because there is nothing else in the payload
// telling Salesforce which record a row belongs to. That is checked here rather
// than left to Salesforce, whose answer to a missing Id is a per-record
// "MISSING_ARGUMENT" that reads like a Flomation bug.
package crm_salesforce_record_update_many

import (
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Many Records"
	Description  = "Update a whole list of existing records on any Salesforce object in one go. Each record needs its Salesforce ID. Up to 200 per API call, split automatically, with an optional roll-everything-back checkbox."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Salesforce Object", Placeholder: "Account, Contact, Opportunity — or a custom one like Invoice__c", Required: true},
	{Name: "records", Type: core.ConnectionTypeObject, Label: "Records", Placeholder: "[{\"Id\":\"0035f000...\",\"Title\":\"Manager\"}] — every record needs its Id", Required: true},
	{Name: "all_or_none", Type: core.ConnectionTypeBoolean, Label: "Roll Everything Back If Any Record Fails"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Fields For Every Record", Placeholder: "{\"Status__c\":\"Reviewed\"} — applied to every record that does not set it itself"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Per-Record Results"},
	{Name: "ids", Type: core.ConnectionTypeObject, Label: "Updated Record IDs"},
	{Name: "success_count", Type: core.ConnectionTypeInteger, Label: "Updated"},
	{Name: "failure_count", Type: core.ConnectionTypeInteger, Label: "Failed"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	object, err := salesforce.RequiredString("object", inputs)
	if err != nil {
		return nil, err
	}
	object, err = salesforce.ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}

	records, err := salesforce.ParseRecordArray("records", inputs)
	if err != nil {
		return nil, err
	}

	// Blanket fields sit UNDER each record so a row's own value wins — see
	// Create Many for the reasoning.
	common := map[string]interface{}{}
	if err := salesforce.MergeAdditionalFields(common, inputs); err != nil {
		return nil, err
	}
	if len(common) > 0 {
		records = applyDefaults(records, common)
	}

	// Check every row carries an Id before spending an API call. Salesforce
	// accepts the request and fails the individual rows, which costs the
	// allowance and produces an error an operator cannot act on.
	for i, rec := range records {
		id := recordID(rec)
		if id == "" {
			return nil, fmt.Errorf("records[%d] has no Id — an update needs the Salesforce ID of the record to change. Use Create Many for new records, or Create or Update Many to match on your own reference", i)
		}
		if err := salesforce.ValidateRecordID(id); err != nil {
			return nil, fmt.Errorf("records[%d]: %w", i, err)
		}
	}

	allOrNone := salesforce.OptionalBool("all_or_none", inputs)
	outcome, err := salesforce.CollectionWrite(instanceURL, token, object, http.MethodPatch, records, allOrNone, "")
	if err != nil {
		// A mid-run failure leaves earlier chunks COMMITTED — report what
		// landed so the operator resumes instead of re-running and duplicating.
		return salesforce.PartialBulkResult(outcome, err, len(records), object), nil
	}

	chunks := len(salesforce.ChunkRecords(records))
	summary := fmt.Sprintf("Updated %d of %d %s record(s) in %d API call(s)", outcome.SuccessNo, len(records), object, chunks)
	if outcome.FailureNo > 0 {
		summary = fmt.Sprintf("Updated %d of %d %s record(s) in %d API call(s); %d failed", outcome.SuccessNo, len(records), object, chunks, outcome.FailureNo)
		if allOrNone && chunks > 1 {
			summary += fmt.Sprintf(" — roll-back applies within each batch of %d, so changes committed by earlier batches were kept", salesforce.MaxCollectionRecords)
		}
	}
	return salesforce.BulkResult(outcome, summary), nil
}

// recordID pulls the record's Salesforce ID out, tolerating the casing a
// spreadsheet or upstream node is likely to produce ("Id", "id", "ID").
func recordID(rec map[string]interface{}) string {
	for k, v := range rec {
		if strings.EqualFold(k, "Id") {
			return strings.TrimSpace(salesforce.StringifyID(v))
		}
	}
	return ""
}

// applyDefaults layers the blanket field values under each record, so a value
// the record supplies itself always wins.
func applyDefaults(records []map[string]interface{}, defaults map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(records))
	for _, rec := range records {
		merged := make(map[string]interface{}, len(defaults)+len(rec))
		for k, v := range defaults {
			merged[k] = v
		}
		for k, v := range rec {
			merged[k] = v
		}
		out = append(out, merged)
	}
	return out
}

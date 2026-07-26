// Package crm_salesforce_record_create_many creates many records of one object
// in a single Salesforce API call.
//
// Why this exists rather than "put Create Record inside a Loop": Salesforce
// charges the org's daily API allowance per REQUEST, not per record, and that
// allowance is shared with every other system touching the CRM. Flomation's
// Loop node is deliberately explicit and easy to reach for, so a 500-row
// spreadsheet import becomes 500 calls — a fifth of a Developer Edition org's
// entire day — unless the platform offers this. sObject Collections takes 200
// records per call, and CollectionWrite chunks automatically, so the operator
// hands over the whole list and never has to know the limit exists.
package crm_salesforce_record_create_many

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Many Records"
	Description  = "Create a whole list of records on any Salesforce object in one go. Up to 200 records per API call, split automatically, so a big import does not eat your org's daily Salesforce allowance."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+layer-group"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Salesforce Object", Placeholder: "Account, Contact, Opportunity — or a custom one like Invoice__c", Required: true},
	{Name: "records", Type: core.ConnectionTypeObject, Label: "Records", Placeholder: "[{\"LastName\":\"Smith\"},{\"LastName\":\"Jones\"}]", Required: true},
	{Name: "all_or_none", Type: core.ConnectionTypeBoolean, Label: "Roll Everything Back If Any Record Fails"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Fields For Every Record", Placeholder: "{\"OwnerId\":\"0055f000...\"} — applied to every record that does not set it itself"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Per-Record Results"},
	{Name: "ids", Type: core.ConnectionTypeObject, Label: "Created Record IDs"},
	{Name: "success_count", Type: core.ConnectionTypeInteger, Label: "Created"},
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

	// "Fields For Every Record" is a DEFAULT, not an override: a value the
	// record carries itself wins. That is the reading an operator expects from
	// "applied to every record", and it lets one blanket OwnerId or
	// RecordTypeId sit alongside a handful of rows that set their own.
	common := map[string]interface{}{}
	if err := salesforce.MergeAdditionalFields(common, inputs); err != nil {
		return nil, err
	}
	if len(common) > 0 {
		records = applyDefaults(records, common)
	}

	allOrNone := salesforce.OptionalBool("all_or_none", inputs)
	outcome, err := salesforce.CollectionWrite(instanceURL, token, object, http.MethodPost, records, allOrNone, "")
	if err != nil {
		// A mid-run failure leaves earlier chunks COMMITTED — report what
		// landed so the operator resumes instead of re-running and duplicating.
		return salesforce.PartialBulkResult(outcome, err, len(records), object), nil
	}

	chunks := len(salesforce.ChunkRecords(records))
	summary := fmt.Sprintf("Created %d of %d %s record(s) in %d API call(s)", outcome.SuccessNo, len(records), object, chunks)
	if outcome.FailureNo > 0 {
		summary = fmt.Sprintf("Created %d of %d %s record(s) in %d API call(s); %d failed", outcome.SuccessNo, len(records), object, chunks, outcome.FailureNo)
		if allOrNone && chunks > 1 {
			// Worth saying out loud: allOrNone is per REQUEST. With more than one
			// chunk an earlier batch has already committed and will not roll back.
			summary += fmt.Sprintf(" — roll-back applies within each batch of %d, so records written by earlier batches were kept", salesforce.MaxCollectionRecords)
		}
	}
	return salesforce.BulkResult(outcome, summary), nil
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

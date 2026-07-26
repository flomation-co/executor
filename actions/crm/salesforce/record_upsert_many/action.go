// Package crm_salesforce_record_upsert_many creates-or-updates many records of
// one object in a single Salesforce API call, matched on an external ID field.
//
// This is the correct shape for a spreadsheet import, a nightly ERP sync or an
// e-commerce catalogue push: the flow can be re-run, replayed after a failure,
// or fired twice by a flapping webhook and the org ends up with one record per
// row every time. Doing the same thing with Create Many produces a duplicate
// set on every run.
//
// One difference from the single-record upsert worth knowing: with sObject
// Collections the external ID field must appear IN each record's body (it is
// the only thing identifying the row), whereas the single-record upsert puts it
// in the URL and Salesforce rejects a body that repeats it.
package crm_salesforce_record_upsert_many

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
	Name         = "Salesforce: Create or Update Many Records"
	Description  = "Import a whole list of records on any Salesforce object, matching each one on a reference of your own — an order number, a customer code, an email. Re-running never creates duplicates. Up to 200 records per API call."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+arrow-right-arrow-left"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Salesforce Object", Placeholder: "Account, Contact, Opportunity — or a custom one like Invoice__c", Required: true},
	{Name: "external_id_field", Type: core.ConnectionTypeString, Label: "Match On Field", Placeholder: "The field Salesforce matches on, e.g. Customer_Ref__c", Required: true},
	{Name: "records", Type: core.ConnectionTypeObject, Label: "Records", Placeholder: "[{\"Customer_Ref__c\":\"CUST-1042\",\"Name\":\"Acme Ltd\"}] — every record needs the match field", Required: true},
	{Name: "all_or_none", Type: core.ConnectionTypeBoolean, Label: "Roll Everything Back If Any Record Fails"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Fields For Every Record", Placeholder: "{\"OwnerId\":\"0055f000...\"} — applied to every record that does not set it itself"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Per-Record Results"},
	{Name: "ids", Type: core.ConnectionTypeObject, Label: "Record IDs"},
	{Name: "success_count", Type: core.ConnectionTypeInteger, Label: "Written"},
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
	extField, err := salesforce.RequiredString("external_id_field", inputs)
	if err != nil {
		return nil, fmt.Errorf("the field to match on is required — pick the external ID field Salesforce should look each record up by")
	}
	extField, err = salesforce.ValidateSOQLFieldName(extField)
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

	// Every row must carry the match field, because on the Collections upsert
	// endpoint that value IS the record's identity — there is no URL segment
	// holding it. A row missing it fails server-side with a message that names
	// neither the row nor the field, so check it here.
	for i, rec := range records {
		if externalIDValue(rec, extField) == "" {
			return nil, fmt.Errorf("records[%d] has no %s value — every record must carry the field being matched on, because that is what tells Salesforce which record the row is", i, extField)
		}
	}

	allOrNone := salesforce.OptionalBool("all_or_none", inputs)
	outcome, err := salesforce.CollectionWrite(instanceURL, token, object, http.MethodPatch, records, allOrNone, extField)
	if err != nil {
		// A mid-run failure leaves earlier chunks COMMITTED — report what
		// landed so the operator resumes instead of re-running and duplicating.
		return salesforce.PartialBulkResult(outcome, err, len(records), object), nil
	}

	chunks := len(salesforce.ChunkRecords(records))
	summary := fmt.Sprintf("Wrote %d of %d %s record(s) matched on %s in %d API call(s)", outcome.SuccessNo, len(records), object, extField, chunks)
	if outcome.FailureNo > 0 {
		summary = fmt.Sprintf("Wrote %d of %d %s record(s) matched on %s in %d API call(s); %d failed", outcome.SuccessNo, len(records), object, extField, chunks, outcome.FailureNo)
		if allOrNone && chunks > 1 {
			summary += fmt.Sprintf(" — roll-back applies within each batch of %d, so records written by earlier batches were kept", salesforce.MaxCollectionRecords)
		}
	}
	return salesforce.BulkResult(outcome, summary), nil
}

// externalIDValue reads a record's match-field value, tolerating the casing
// difference between a spreadsheet header and the Salesforce API name.
func externalIDValue(rec map[string]interface{}, field string) string {
	for k, v := range rec {
		if strings.EqualFold(k, field) {
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

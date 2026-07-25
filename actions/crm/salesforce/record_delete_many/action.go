// Package crm_salesforce_record_delete_many deletes many records in a single
// Salesforce API call.
//
// Two things worth knowing before using it. Salesforce IDs are self-describing
// — the first three characters encode the object — so the delete endpoint takes
// a plain list of IDs and no object name; a list may even mix objects. And a
// delete here is a move to the Recycle Bin, not destruction: for the next 15
// days the records can be brought back with Restore Record.
package crm_salesforce_record_delete_many

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Many Records"
	Description  = "Delete a list of Salesforce records in one go — up to 200 per API call, split automatically. Deleted records go to the Recycle Bin and can be restored for 15 days."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "record_ids", Type: core.ConnectionTypeString, Label: "Record IDs", Placeholder: "0035f000...,0035f000... — comma separated, or a list from an earlier step", Required: true},
	{Name: "all_or_none", Type: core.ConnectionTypeBoolean, Label: "Roll Everything Back If Any Record Fails"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Per-Record Results"},
	{Name: "ids", Type: core.ConnectionTypeObject, Label: "Deleted Record IDs"},
	{Name: "success_count", Type: core.ConnectionTypeInteger, Label: "Deleted"},
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

	raw, err := salesforce.RequiredString("record_ids", inputs)
	if err != nil {
		return nil, fmt.Errorf("record_ids is required — the Salesforce IDs of the records to delete")
	}
	ids, err := parseIDs(raw)
	if err != nil {
		return nil, err
	}
	// Validate before spending an API call: one malformed ID fails the whole
	// chunk server-side, and Salesforce's MALFORMED_ID does not say which of
	// the 200 it objected to.
	for i, id := range ids {
		if err := salesforce.ValidateRecordID(id); err != nil {
			return nil, fmt.Errorf("record %d: %w", i+1, err)
		}
	}

	allOrNone := salesforce.OptionalBool("all_or_none", inputs)
	outcome, err := salesforce.CollectionDelete(instanceURL, token, ids, allOrNone)
	if err != nil {
		// Earlier chunks are already deleted and will not come back on their
		// own; say which, so the operator knows what is left rather than
		// re-running a delete over records that no longer exist.
		return salesforce.PartialBulkResult(outcome, err, len(ids), "record"), nil
	}

	chunks := (len(ids) + salesforce.MaxCollectionRecords - 1) / salesforce.MaxCollectionRecords
	summary := fmt.Sprintf("Deleted %d of %d record(s) in %d API call(s) — they are in the Recycle Bin for 15 days", outcome.SuccessNo, len(ids), chunks)
	if outcome.FailureNo > 0 {
		summary = fmt.Sprintf("Deleted %d of %d record(s) in %d API call(s); %d failed", outcome.SuccessNo, len(ids), chunks, outcome.FailureNo)
		if allOrNone && chunks > 1 {
			// allOrNone is per REQUEST: with more than one chunk the earlier
			// batches have already committed and are not rolled back.
			summary += fmt.Sprintf(" — roll-back applies within each batch of %d, so deletions in earlier batches stand", salesforce.MaxCollectionRecords)
		}
	}
	return salesforce.BulkResult(outcome, summary), nil
}

// parseIDs reads the Record IDs input, which arrives either as the
// comma-separated text an operator types or as the JSON array an upstream
// action's "ids" output produces. Accepting both is what lets Get Deleted
// Records or a query feed straight into this action without a Code node in
// between.
func parseIDs(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "[") {
		var arr []interface{}
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return nil, fmt.Errorf("record_ids looks like a list but is not valid JSON: %w", err)
		}
		ids := make([]string, 0, len(arr))
		for _, item := range arr {
			if id := strings.TrimSpace(salesforce.StringifyID(item)); id != "" {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("record_ids is an empty list — pass at least one Salesforce record ID")
		}
		return ids, nil
	}
	ids := salesforce.SplitList(trimmed)
	if len(ids) == 0 {
		return nil, fmt.Errorf("record_ids is required — the Salesforce IDs of the records to delete")
	}
	return ids, nil
}

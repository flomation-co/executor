// Package crm_salesforce_approval_submit sends a Salesforce record into an
// approval process.
//
// This is the automation half of "a discount over 20% needs the manager to sign
// it off": the flow decides the rule has been tripped and hands the record to
// the approval process the Salesforce admin already built, rather than trying to
// reimplement sign-off logic outside the CRM. Salesforce then does the rest —
// locking the record, emailing the approver, and showing the approval history
// on the record.
//
// The endpoint is POST /process/approvals/, which is shared by Submit, Approve
// and Reject and is unusual in three ways:
//
//   - The request is {"requests":[{...}]} and the response is a JSON ARRAY of
//     per-request results, one per entry sent. This action sends exactly one.
//   - A REFUSED submission can come back HTTP 200 with success:false and a
//     populated errors array, so the status code alone is not enough.
//   - nextApproverIds is only accepted when the process step is configured to
//     let the submitter choose the approver manually. Supplying it against a
//     process that assigns approvers automatically is rejected — which is why
//     the field is optional here and its help text says so.
package crm_salesforce_approval_submit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Submit for Approval"
	Description  = "Send a Salesforce record into an approval process so the right person is asked to approve it."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+share-from-square"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Object", Placeholder: "Opportunity — narrows the record picker"},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record to Submit", Placeholder: "0065f00000AbcDEAA", Required: true},
	{Name: "approval_process_name", Type: core.ConnectionTypeString, Label: "Approval Process", Placeholder: "Discount_Approval — leave empty to let Salesforce choose"},
	{Name: "next_approver_id", Type: core.ConnectionTypeString, Label: "Send To (Approver)", Placeholder: "0055f00000AbcDEAA — only if the process lets the submitter pick"},
	{Name: "submitter_id", Type: core.ConnectionTypeString, Label: "Submit On Behalf Of", Placeholder: "0055f00000AbcDEAA — defaults to the connected user"},
	{Name: "comments", Type: core.ConnectionTypeText, Label: "Comments", Placeholder: "Discount is 25% — approved verbally by the customer's account manager"},
	{Name: "skip_entry_criteria", Type: core.ConnectionTypeBoolean, Label: "Ignore the Process Entry Criteria"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"processDefinitionNameOrId\":\"Discount_Approval\"} — any other approval request field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Approval Request ID (use this to approve or reject)"},
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

	recordID := salesforce.OptionalString("record_id", inputs)
	if recordID == "" {
		return nil, fmt.Errorf("Record to Submit is required — pick the record that needs approving")
	}
	if err := salesforce.ValidateRecordID(recordID); err != nil {
		return nil, fmt.Errorf("Record to Submit: %w", err)
	}
	// The Object only narrows the record picker in the editor; the approval
	// endpoint works out the object from the record ID itself. Validating it
	// still catches a typo that would otherwise leave an empty picker.
	if object := salesforce.OptionalString("object", inputs); object != "" {
		if _, err := salesforce.ValidateSOQLObjectName(object); err != nil {
			return nil, fmt.Errorf("Object: %w", err)
		}
	}

	request := map[string]interface{}{
		"actionType": "Submit",
		"contextId":  recordID,
	}
	salesforce.SetIfPresent(request, inputs, "comments", "comments")
	salesforce.SetIfPresent(request, inputs, "processDefinitionNameOrId", "approval_process_name")
	salesforce.SetBoolIfSet(request, inputs, "skipEntryCriteria", "skip_entry_criteria")

	if submitterID := salesforce.OptionalString("submitter_id", inputs); submitterID != "" {
		if err := salesforce.ValidateRecordID(submitterID); err != nil {
			return nil, fmt.Errorf("Submit On Behalf Of: %w", err)
		}
		request["submitterId"] = submitterID
	}

	// nextApproverIds is an ARRAY even for the one approver this input usually
	// holds; a comma-separated list is accepted for the rare multi-approver
	// step so the operator never has to hand-write JSON for it.
	approvers := salesforce.SplitList(salesforce.OptionalString("next_approver_id", inputs))
	if len(approvers) > 0 {
		for _, approver := range approvers {
			if err := salesforce.ValidateRecordID(approver); err != nil {
				return nil, fmt.Errorf("Send To (Approver): %w", err)
			}
		}
		request["nextApproverIds"] = toInterfaceSlice(approvers)
	}

	if err := salesforce.MergeAdditionalFields(request, inputs); err != nil {
		return nil, err
	}

	entry, err := postApprovalRequest(instanceURL, token, request)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if failed, msg := approvalFailed(entry); failed {
		return salesforce.ErrorResult("Salesforce did not submit the record for approval: " + msg), nil
	}

	// The work item is the thing the next step in the flow needs: approving or
	// rejecting later is addressed by work item ID, not by the record ID or the
	// process instance. Surfacing it as the primary output is what makes
	// "submit → ask in Slack → approve" wire up without digging into the raw
	// response.
	workItemID := firstString(entry["newWorkitemIds"])
	id := workItemID
	if id == "" {
		id = stringValue(entry["instanceId"])
	}

	status := stringValue(entry["instanceStatus"])
	if status == "" {
		status = "Pending"
	}
	summary := fmt.Sprintf("Submitted record %s for approval — status %s", recordID, status)
	if actors := stringList(entry["actorIds"]); len(actors) > 0 {
		summary += ", waiting on " + strings.Join(actors, ", ")
	}
	return salesforce.RecordResult(id, entry, summary), nil
}

// postApprovalRequest sends one approval request and returns its per-request
// result. Salesforce takes a list and answers with a list; this action always
// sends one entry, so the first result is the only result.
func postApprovalRequest(instanceURL, token string, request map[string]interface{}) (map[string]interface{}, error) {
	// The trailing slash is Salesforce's own — /process/approvals without it
	// is not the same resource.
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodPost, "/process/approvals/", map[string]interface{}{
		"requests": []interface{}{request},
	})
	if err != nil {
		return nil, err
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return nil, err
	}
	var results []map[string]interface{}
	if err := json.Unmarshal(resp.Body, &results); err != nil {
		return nil, fmt.Errorf("failed to parse the Salesforce approval response: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("Salesforce accepted the request but returned no approval result")
	}
	return results[0], nil
}

// approvalFailed reports whether a per-request result says the approval action
// was refused, with a readable reason.
//
// A refusal is NOT an HTTP error: Salesforce answers 200 with success:false for
// the everyday cases (the record is already in an approval process, no approver
// could be worked out, the record failed the entry criteria). Trusting the
// status code alone would report those as successes.
func approvalFailed(entry map[string]interface{}) (bool, string) {
	ok, present := entry["success"].(bool)
	if !present || ok {
		return false, ""
	}
	msgs := []string{}
	for _, raw := range asSlice(entry["errors"]) {
		e, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		msg, _ := e["message"].(string)
		// Approval errors carry statusCode where the rest of the REST API uses
		// errorCode, so neither key can be assumed present.
		code, _ := e["statusCode"].(string)
		if code == "" {
			code, _ = e["errorCode"].(string)
		}
		switch {
		case msg != "" && code != "":
			msgs = append(msgs, msg+" ("+code+")")
		case msg != "":
			msgs = append(msgs, msg)
		case code != "":
			msgs = append(msgs, code)
		}
	}
	if len(msgs) == 0 {
		return true, "Salesforce gave no reason — check the record meets the process entry criteria and is not already awaiting approval"
	}
	return true, strings.Join(msgs, "; ")
}

// asSlice normalises a JSON value that may be a list, a single object or null
// into a list, so callers can range over it unconditionally.
func asSlice(v interface{}) []interface{} {
	switch t := v.(type) {
	case []interface{}:
		return t
	case map[string]interface{}:
		return []interface{}{t}
	}
	return nil
}

// stringValue renders a JSON scalar as a clean string.
func stringValue(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return salesforce.StringifyID(v)
}

// stringList renders a JSON array of IDs as strings, skipping empties.
func stringList(v interface{}) []string {
	items := asSlice(v)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := stringValue(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// firstString returns the first entry of a JSON array of IDs, or "".
func firstString(v interface{}) string {
	if list := stringList(v); len(list) > 0 {
		return list[0]
	}
	return ""
}

// toInterfaceSlice widens a string slice for JSON encoding.
func toInterfaceSlice(values []string) []interface{} {
	out := make([]interface{}, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

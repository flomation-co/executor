// Package crm_salesforce_approval_approve approves a pending Salesforce
// approval request.
//
// It is the other end of the human-in-the-loop pattern: a flow submitted a
// record for approval, asked the manager in Slack or Teams, and this action
// records their "yes" back in Salesforce so the process advances, the record
// unlocks, and the approval history shows who signed it off and when.
//
// The one thing that trips people up is WHICH ID to pass. It is not the record
// ID and not the approval process ID — it is the approval request (work item)
// ID, which the Submit for Approval action returns as its main output. Salesforce
// calls it contextId on the wire, which helps nobody, so this input is labelled
// for what it actually is.
//
// The endpoint is POST /process/approvals/ shared with Submit and Reject: the
// request is {"requests":[{...}]}, the response is an ARRAY of per-request
// results, and a refusal (already approved, not your approval to give) arrives
// as HTTP 200 with success:false rather than an error status.
package crm_salesforce_approval_approve

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
	Name         = "Salesforce: Approve Request"
	Description  = "Approve a Salesforce record that is waiting for approval, with an optional comment recorded against the decision."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+circle-check"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "work_item_id", Type: core.ConnectionTypeString, Label: "Approval Request", Placeholder: "04i5f00000AbcDEAA — from Submit for Approval, or Get Many Approval Processes", Required: true},
	{Name: "comments", Type: core.ConnectionTypeText, Label: "Comments", Placeholder: "Approved — discount is within the agreed range for this account"},
	{Name: "next_approver_id", Type: core.ConnectionTypeString, Label: "Send To (Next Approver)", Placeholder: "0055f00000AbcDEAA — only if the next step lets you pick"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"comments\":\"Approved\"} — any other approval request field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Approval Request ID"},
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

	workItemID := salesforce.OptionalString("work_item_id", inputs)
	if workItemID == "" {
		return nil, fmt.Errorf("Approval Request is required — pass the approval request ID returned by Submit for Approval")
	}
	if err := salesforce.ValidateRecordID(workItemID); err != nil {
		return nil, fmt.Errorf("Approval Request: %w", err)
	}

	request := map[string]interface{}{
		"actionType": "Approve",
		"contextId":  workItemID,
	}
	salesforce.SetIfPresent(request, inputs, "comments", "comments")

	// A multi-step process can hand on to another approver, and where that step
	// is set to "let the approver choose" Salesforce expects the next approver
	// here. It is an ARRAY on the wire even for the single approver this input
	// normally holds.
	approvers := salesforce.SplitList(salesforce.OptionalString("next_approver_id", inputs))
	if len(approvers) > 0 {
		for _, approver := range approvers {
			if err := salesforce.ValidateRecordID(approver); err != nil {
				return nil, fmt.Errorf("Send To (Next Approver): %w", err)
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
		return salesforce.ErrorResult("Salesforce did not approve the request: " + msg), nil
	}

	// instanceStatus is the state of the whole approval AFTER this decision:
	// "Approved" when this was the last step, "Pending" when it has moved on to
	// another approver. That distinction is the thing an operator wants in the
	// run history, so it goes in the summary rather than only the raw response.
	status := stringValue(entry["instanceStatus"])
	summary := fmt.Sprintf("Approved request %s", workItemID)
	if entityID := stringValue(entry["entityId"]); entityID != "" {
		summary += " for record " + entityID
	}
	switch status {
	case "Approved":
		summary += " — the approval is now complete"
	case "Pending":
		summary += " — it has moved on to the next approver"
	case "":
		// Nothing useful to add; leave the summary as it is.
	default:
		summary += " — approval status " + status
	}
	return salesforce.RecordResult(workItemID, entry, summary), nil
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
// the everyday cases (the request has already been decided, or the connected
// user is not the assigned approver). Trusting the status code alone would
// report those as successes.
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
		return true, "Salesforce gave no reason — check the request is still pending and the connected user is the assigned approver"
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

// toInterfaceSlice widens a string slice for JSON encoding.
func toInterfaceSlice(values []string) []interface{} {
	out := make([]interface{}, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

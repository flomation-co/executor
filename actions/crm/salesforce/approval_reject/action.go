// Package crm_salesforce_approval_reject rejects a pending Salesforce approval
// request.
//
// The mirror of Approve Request, and the reason the comment field matters more
// here than anywhere else in this node: a rejection without a reason sends the
// record back to whoever submitted it with no explanation, and they will simply
// resubmit it. The comment is recorded in the approval history, so "priced
// below floor, resubmit at 15%" reaches the person who has to act on it.
//
// The ID to pass is the approval request (work item) ID — the main output of
// Submit for Approval — not the record ID and not the approval process ID.
// Salesforce calls it contextId on the wire.
//
// Unlike Approve, there is deliberately no next-approver input: a rejection
// ends the current step rather than handing on, and Salesforce rejects a
// nextApproverIds sent alongside a Reject.
//
// The endpoint is POST /process/approvals/ shared with Submit and Approve: the
// request is {"requests":[{...}]}, the response is an ARRAY of per-request
// results, and a refusal arrives as HTTP 200 with success:false rather than an
// error status.
package crm_salesforce_approval_reject

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
	Name         = "Salesforce: Reject Request"
	Description  = "Reject a Salesforce record that is waiting for approval, recording the reason against the decision."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+xmark"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "work_item_id", Type: core.ConnectionTypeString, Label: "Approval Request", Placeholder: "04i5f00000AbcDEAA — from Submit for Approval, or Get Many Approval Processes", Required: true},
	{Name: "comments", Type: core.ConnectionTypeText, Label: "Reason for Rejecting", Placeholder: "Priced below our floor — resubmit at 15% or less"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"comments\":\"Rejected\"} — any other approval request field"},
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
		"actionType": "Reject",
		"contextId":  workItemID,
	}
	salesforce.SetIfPresent(request, inputs, "comments", "comments")

	if err := salesforce.MergeAdditionalFields(request, inputs); err != nil {
		return nil, err
	}

	entry, err := postApprovalRequest(instanceURL, token, request)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if failed, msg := approvalFailed(entry); failed {
		return salesforce.ErrorResult("Salesforce did not reject the request: " + msg), nil
	}

	// instanceStatus after a rejection is normally "Rejected", but a process
	// configured to send a rejection back a step can leave it Pending — worth
	// saying out loud rather than letting the operator assume it is finished.
	status := stringValue(entry["instanceStatus"])
	summary := fmt.Sprintf("Rejected request %s", workItemID)
	if entityID := stringValue(entry["entityId"]); entityID != "" {
		summary += " for record " + entityID
	}
	switch status {
	case "Rejected":
		summary += " — the record has been sent back to whoever submitted it"
	case "Pending":
		summary += " — the approval has gone back a step and is pending again"
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

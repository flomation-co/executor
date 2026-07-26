package crm_salesforce_event_delete_series

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Cancel Event Series"
	Description  = "Cancel a repeating Salesforce appointment from one occurrence onwards — the standing weekly check-in that stops when a customer leaves. Earlier occurrences stay in the calendar as a record of what happened."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+xmark"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// fromThisEventOnwards is Salesforce's series-delete suffix. It is a separate
// resource rather than a flag on the ordinary delete, and it is the only way to
// clear the rest of a repeating series in one call — without it an operator
// deletes occurrences one at a time, which is exactly the chore this removes.
const fromThisEventOnwards = "/fromThisEventOnwards"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Cancel From This Event Onwards", Placeholder: "00U5f00000AbCdEEAV — the first occurrence to cancel", Required: true},
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

	// The shared DeleteRecord helper targets /sobjects/Event/{id}; this resource
	// hangs one path segment below it, so the call is made directly. The ID is
	// path-escaped for the same reason it is everywhere else in this node.
	path := "/sobjects/Event/" + url.PathEscape(eventID) + fromThisEventOnwards
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodDelete, path, nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Like every Salesforce delete this answers 204 No Content, so the ID we were
	// given is the only thing there is to hand on.
	summary := fmt.Sprintf("Cancelled event %s and every later occurrence in its series", eventID)
	return salesforce.RecordResult(eventID, nil, summary), nil
}

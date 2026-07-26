// Package crm_salesforce_contact_describe reads the Contact object's own
// metadata rather than any contact record: what the object is called in this
// org, what can be done to it, and which contacts this user looked at recently.
//
// It doubles as the cheapest possible connection check — one call that proves
// the token works, the instance URL is right and the connected user can see
// Contacts at all, without touching customer data.
package crm_salesforce_contact_describe

import (
	"encoding/json"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Contact Metadata"
	Description  = "Fetch details about the Contact object in your org — what it is called, what the connected user may do with it, and the contacts they viewed recently. Useful as a quick connection check."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clipboard-list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	// Off by default because the full describe is a large payload (hundreds of
	// kilobytes on an org with many custom fields) and most flows only want the
	// summary and the recent items.
	{Name: "include_field_details", Type: core.ConnectionTypeBoolean, Label: "Include Every Field and Picklist"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Contact Metadata"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// GET /sobjects/Contact is the sObject BASIC INFORMATION resource, not the
	// full describe: it answers with {objectDescribe, recentItems}. The heavy
	// /describe payload is fetched separately, and only when asked for.
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/Contact", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	var info map[string]interface{}
	if err := json.Unmarshal(resp.Body, &info); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("could not read the Salesforce response: %v", err)), nil
	}
	// A literal JSON "null" unmarshals cleanly into a NIL map, and writing the
	// describe into a nil map below would panic rather than fail. Cheap guard.
	if info == nil {
		info = map[string]interface{}{}
	}

	fieldCount := 0
	if salesforce.OptionalBool("include_field_details", inputs) {
		// Worth knowing: describe is filtered by the CONNECTED user's field-level
		// security, so a field the administrator can see may simply be absent
		// here for the user whose token actually runs the flow.
		describe, err := salesforce.DescribeObject(instanceURL, token, "Contact")
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		info["describe"] = describe
		if fields, ok := describe["fields"].([]interface{}); ok {
			fieldCount = len(fields)
		}
	}

	recent := 0
	if items, ok := info["recentItems"].([]interface{}); ok {
		recent = len(items)
	}
	label := "Contact"
	if describe, ok := info["objectDescribe"].(map[string]interface{}); ok {
		if l, ok := describe["label"].(string); ok && l != "" {
			label = l
		}
	}

	summary := fmt.Sprintf("Read metadata for %s — %d recently viewed", label, recent)
	if fieldCount > 0 {
		summary = fmt.Sprintf("Read metadata for %s — %d field(s), %d recently viewed", label, fieldCount, recent)
	}
	// The record ID slot carries the object name here: there is no record to
	// point at, and the object name is the identifier a downstream step would
	// actually reuse.
	return salesforce.RecordResult("Contact", info, summary), nil
}

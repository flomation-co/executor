// Package crm_salesforce_attachment_describe reports what the Attachment object
// looks like in this org, plus the attachments the connected user opened most
// recently.
//
// n8n calls this operation "getSummary", which is actively misleading: it takes
// no ID and summarises no record. It is Salesforce's sObject Basic Information
// call — object metadata and a recently-viewed list — so it is named for what
// it does.
//
// It is genuinely useful for two things: confirming the connected user can see
// Attachment at all before a flow tries to write one, and discovering the field
// names to put in a Fields to Return box.
package crm_salesforce_attachment_describe

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
	Name         = "Salesforce: Describe Attachments (Classic)"
	Description  = "Check what your org stores on a Classic attachment and list the attachments you opened most recently — handy for finding the right field names."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},

	// Off by default: the full describe is a large payload, and the basic call
	// already answers "can I see this object and what have I touched lately".
	{Name: "include_fields", Type: core.ConnectionTypeBoolean, Label: "Include Every Field (larger response)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Object Details"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// /sobjects/Attachment with no ID and no /describe suffix is the Basic
	// Information call: {objectDescribe, recentItems}. It is not the same
	// endpoint as the full describe below, and it is much cheaper.
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/Attachment", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse the Salesforce response: %v", err)), nil
	}
	// A literal "null" body unmarshals to a nil map without erroring, and the
	// describe merge below writes into it — which would panic rather than fail.
	if result == nil {
		result = map[string]interface{}{}
	}

	if salesforce.OptionalBool("include_fields", inputs) {
		// Worth knowing: describe is filtered by the CONNECTED user's
		// permissions. A field the admin who built the flow can see may simply
		// be absent for the user whose token actually runs it.
		describe, err := salesforce.DescribeObject(instanceURL, token, "Attachment")
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		result["fields"] = describe["fields"]
		result["recordTypeInfos"] = describe["recordTypeInfos"]
	}

	recent, _ := result["recentItems"].([]interface{})
	return salesforce.RecordResult("Attachment", result,
		fmt.Sprintf("Read the Attachment object details and %d recently-viewed attachment(s)", len(recent))), nil
}

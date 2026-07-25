// Package crm_salesforce_lead_describe reports what the connected Salesforce
// user can do with the Lead object, plus the leads they have looked at recently.
//
// n8n calls this operation "Get Summary", which reads like a count or a report
// and is neither — it is the sObject Basic Information resource. Renamed here to
// say what it actually returns, because an operator who picks it expecting
// numbers gets metadata and no explanation.
//
// The honest use for it is a connection sanity check: it is the cheapest call
// that proves the token works, the org can see Lead, and whether this user is
// allowed to create or update one. Everything it reports is filtered by that
// user's permissions, so a flow that works for the admin who built it can report
// differently for the account that actually runs it.
package crm_salesforce_lead_describe

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
	Name         = "Salesforce: Describe Lead Object"
	Description  = "Check what your Salesforce connection is allowed to do with leads, and list the leads this user has viewed recently. Useful for confirming a connection works."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clipboard-list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Lead Object Information"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// GET on the object root (not /describe) is the Basic Information resource:
	// a small metadata block plus recentItems. The full field-level describe is
	// far larger and is what powers the dropdowns elsewhere in this node, so it
	// is deliberately not what this action returns.
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/Lead", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	var info map[string]interface{}
	if err := json.Unmarshal(resp.Body, &info); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("Salesforce returned something unexpected for the Lead object: %v", err)), nil
	}

	// RecordResult would otherwise dig an "Id" out of the body; there is no
	// record here, so name the object explicitly and keep the output shape the
	// same as every other single-result Salesforce action.
	out := salesforce.RecordResult("Lead", info, summarise(info))
	return out, nil
}

// summarise turns the metadata block into one line an operator can read without
// knowing what "createable" means.
func summarise(info map[string]interface{}) string {
	describe, _ := info["objectDescribe"].(map[string]interface{})
	label := "Lead"
	if describe != nil {
		if l, ok := describe["label"].(string); ok && l != "" {
			label = l
		}
	}

	recent, _ := info["recentItems"].([]interface{})

	permissions := "read only"
	if boolField(describe, "createable") && boolField(describe, "updateable") {
		permissions = "create and update allowed"
	} else if boolField(describe, "createable") {
		permissions = "create allowed"
	} else if boolField(describe, "updateable") {
		permissions = "update allowed"
	}

	return fmt.Sprintf("Connected to the %s object — %s; %d recently viewed", label, permissions, len(recent))
}

func boolField(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	v, _ := m[key].(bool)
	return v
}

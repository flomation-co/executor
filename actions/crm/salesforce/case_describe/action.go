// Package crm_salesforce_case_describe reports what the connected Salesforce
// user can do with the Case object, plus the cases they have looked at recently.
//
// n8n calls this operation "Get Summary", which reads like a count or a report
// and is neither — it is the sObject Basic Information resource. Renamed here to
// say what it actually returns, because an operator who picks it expecting
// numbers gets metadata and no explanation.
//
// The honest use for it is a connection sanity check: it is the cheapest call
// that proves the token works, the org can see Case, and whether this user is
// allowed to create or update one. Everything it reports is filtered by that
// user's permissions, so a flow that works for the admin who built it can report
// differently for the account that actually runs it.
//
// It also answers the single most common Case support question, behind an
// opt-in switch: "which Status / Type / Priority values does MY org accept?"
// Those five picklists are org-editable, they gate every other case action, and
// getting one wrong is what produces an unhelpful restricted-picklist rejection.
package crm_salesforce_case_describe

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
	Name         = "Salesforce: Describe Case Object"
	Description  = "Check what your Salesforce connection is allowed to do with cases, and list the cases this user has viewed recently. Optionally list the Status, Priority, Type, Origin and Reason values your org actually accepts."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clipboard-list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// casePicklists are the five Case fields every other action in this group asks
// the operator to type a value into. They are listed together because they fail
// together: each one is an org-editable picklist, and a value that is right in
// one Salesforce org is rejected in the next.
var casePicklists = []string{"Status", "Priority", "Type", "Origin", "Reason"}

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	// Off by default: the full field-level describe is a large payload and a
	// second call against the org's daily API allowance. Worth it when you are
	// setting a flow up and need to know what to type into Status.
	{Name: "include_picklist_values", Type: core.ConnectionTypeBoolean, Label: "List the Status, Priority, Type, Origin and Reason Values"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Case Object Information"},
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
	// is deliberately not what this action returns by default.
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/Case", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	var info map[string]interface{}
	if err := json.Unmarshal(resp.Body, &info); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("Salesforce returned something unexpected for the Case object: %v", err)), nil
	}

	picklists := 0
	if salesforce.OptionalBool("include_picklist_values", inputs) {
		values, err := collectPicklists(instanceURL, token)
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		info["picklistValues"] = values
		picklists = len(values)
	}

	// RecordResult would otherwise dig an "Id" out of the body; there is no
	// record here, so name the object explicitly and keep the output shape the
	// same as every other single-result Salesforce action.
	return salesforce.RecordResult("Case", info, summarise(info, picklists)), nil
}

// collectPicklists reads the org's active values for the five Case picklists.
//
// One describe call covers all five. n8n fires a separate describe per dropdown
// — five identical round trips through a large payload if an operator opens
// every list — which is exactly the mistake worth not copying.
//
// A field that is missing, or is not a picklist in this org, is skipped rather
// than reported as empty: PicklistValues returns nil for both cases and an
// empty list would read as "your org accepts nothing here", which is wrong.
func collectPicklists(instanceURL, token string) (map[string]interface{}, error) {
	describe, err := salesforce.DescribeObject(instanceURL, token, "Case")
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{}
	for _, field := range casePicklists {
		values := salesforce.PicklistValues(describe, field)
		if len(values) == 0 {
			continue
		}
		items := make([]interface{}, 0, len(values))
		for _, v := range values {
			items = append(items, map[string]interface{}{
				"label":     v["label"],
				"value":     v["value"],
				"isDefault": v["defaultValue"],
			})
		}
		out[field] = items
	}
	return out, nil
}

// summarise turns the metadata block into one line an operator can read without
// knowing what "createable" means.
func summarise(info map[string]interface{}, picklists int) string {
	describe, _ := info["objectDescribe"].(map[string]interface{})
	label := "Case"
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

	summary := fmt.Sprintf("Connected to the %s object — %s; %d recently viewed", label, permissions, len(recent))
	if picklists > 0 {
		summary += fmt.Sprintf("; listed the values for %d picklist(s)", picklists)
	}
	return summary
}

func boolField(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	v, _ := m[key].(bool)
	return v
}

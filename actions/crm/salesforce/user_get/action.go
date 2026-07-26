// Package crm_salesforce_user_get reads one Salesforce User record.
//
// User is an ordinary sObject, so this is the standard GET
// /sobjects/User/{id} — no special casing needed. What is worth knowing is why
// a flow reads one: a lead, case or task carries an OwnerId, a CreatedById, a
// ManagerId, and none of those tell you anything a human can act on. Resolving
// the ID to a name, email and (crucially) whether the login is still active is
// what turns "assigned to 0055f00000AbCdEAAV" into "assigned to Jane Smith, who
// left in March".
package crm_salesforce_user_get

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get User"
	Description  = "Look up one Salesforce user by their record ID — their name, email, job title, profile and whether their login is still active."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID", Placeholder: "0055f00000AbCdEAAV — every Salesforce user ID starts with 005", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields to Return", Placeholder: "Leave blank for the usual ones, or list them: Name, Email, IsActive, Title"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	userID, err := salesforce.RequiredString("user_id", inputs)
	if err != nil {
		return nil, err
	}
	// Check the ID shape here rather than letting Salesforce answer MALFORMED_ID.
	// A mistyped ID is a configuration mistake, so it belongs on the hard-failure
	// path where the operator sees it while they are still editing the node.
	if err := salesforce.ValidateRecordID(userID); err != nil {
		return nil, err
	}

	// The field list is also configuration, so validate it up front. GetRecord
	// re-validates, but doing it here keeps a typo'd field name off the error
	// port (where it would look like a Salesforce outage) and leaves every
	// remaining failure below genuinely provider-side.
	fields := salesforce.OptionalString("fields", inputs)
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, err
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "User", userID, fields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	return salesforce.RecordResult(userID, record, describeUser(record, userID)), nil
}

// describeUser renders the one-line summary an operator reads in the run log.
// The requested field list may have excluded Name or Username, so every part is
// optional and the record ID is the guaranteed fallback.
func describeUser(record map[string]interface{}, userID string) string {
	name := textField(record, "Name")
	username := textField(record, "Username")

	var label string
	switch {
	case name != "" && username != "":
		label = fmt.Sprintf("%s (%s)", name, username)
	case name != "":
		label = name
	case username != "":
		label = username
	default:
		label = userID
	}

	// IsActive is the single most useful thing about a user record — a flow that
	// looks a person up is usually deciding whether they still work here.
	if active, ok := record["IsActive"].(bool); ok && !active {
		return "Retrieved " + label + " — this login is deactivated"
	}
	return "Retrieved " + label
}

// textField reads a string field from a record, tolerating the JSON nulls
// Salesforce returns for fields the user has not filled in.
func textField(record map[string]interface{}, key string) string {
	v, ok := record[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

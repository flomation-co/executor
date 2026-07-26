// Package crm_salesforce_user_deactivate switches off a leaver's Salesforce
// login.
//
// The leaver half of joiners-movers-leavers, and the one with teeth: a
// Salesforce user record can NEVER be deleted, only deactivated, because
// everything they ever touched still points at them as owner, creator or last
// modifier. Deactivating is therefore the whole of offboarding as far as
// Salesforce is concerned — the login stops working, the licence is released,
// and every record they own stays exactly where it is.
//
// Today that is a manual click in Setup that gets forgotten, which is how ex-
// employees keep working CRM access. Automating it off the HR system's leaver
// event is the point of this action, and why it is written to be safe to re-run:
// a retried webhook must not look like a second change in the audit trail.
package crm_salesforce_user_deactivate

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Deactivate User"
	Description  = "Switch off a leaver's Salesforce login and free up their licence. Salesforce users can never be deleted, only deactivated, and their records stay exactly where they are."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-minus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// lookupFields is the minimal read taken before the write. Kept to four fields
// so the extra call is as cheap as possible against the org's API allowance.
const lookupFields = "Id,Name,Username,IsActive"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID", Placeholder: "0055f00000AbCdEAAV — every Salesforce user ID starts with 005", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deactivated User"},
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
	if err := salesforce.ValidateRecordID(userID); err != nil {
		return nil, err
	}

	// Read the user before writing, for two reasons worth the extra call on an
	// action that runs once per leaver:
	//
	//   - Offboarding flows get re-run (a retried HR webhook, a re-imported
	//     starters-and-leavers sheet). Deactivating someone already deactivated
	//     should report that plainly, not look like a fresh change in the audit
	//     trail.
	//   - This is a security step someone will be asked to evidence later, and
	//     "Deactivated Jane Smith (jane.smith@acme.com)" is evidence in a way
	//     that a bare record ID is not.
	record, err := salesforce.GetRecord(instanceURL, token, "User", userID, lookupFields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	label := describeUser(record, userID)

	// Already off: nothing to write, so say so and leave the record untouched.
	// This is what makes the action safe to re-run.
	if active, ok := record["IsActive"].(bool); ok && !active {
		return salesforce.RecordResult(userID, record, "No change — "+label+" was already deactivated"), nil
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "User", userID, map[string]interface{}{"IsActive": false}); err != nil {
		// Salesforce blocks deactivation while the person is still wired into the
		// org — a default lead or case owner, an active approval delegate, the
		// owner of a scheduled report. CheckResponse surfaces which; the operator
		// reassigns and re-runs, so this belongs on the error port as data rather
		// than as a dead flow.
		return salesforce.ErrorResult(err.Error()), nil
	}

	// 204 No Content on success, so shape the record we already read rather than
	// returning nothing a downstream node could use.
	record["IsActive"] = false
	return salesforce.RecordResult(userID, record, "Deactivated "+label+" — their licence is freed and their records are unchanged"), nil
}

// describeUser names the person for the summary, falling back through Name,
// Username and finally the record ID so the line always reads sensibly.
func describeUser(record map[string]interface{}, userID string) string {
	name := textField(record, "Name")
	username := textField(record, "Username")

	switch {
	case name != "" && username != "":
		return fmt.Sprintf("%s (%s)", name, username)
	case name != "":
		return name
	case username != "":
		return username
	default:
		return "user " + userID
	}
}

// textField reads a string field from a record, tolerating the JSON nulls
// Salesforce returns for anything the user has not filled in.
func textField(record map[string]interface{}, key string) string {
	v, ok := record[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

// Package crm_salesforce_case_close closes a support Case.
//
// Closing a case is a status change, so on paper this is just an update — which
// is why n8n has no such operation. In practice it is the one status change an
// operator cannot make safely, because there is no fixed value to type: the
// Status picklist is org-editable and support teams rename their closed state
// ("Resolved", "Completed", "Closed - No Response") or run several of them. A
// flow with the literal "Closed" hard-coded in an update node works in the org
// it was built in and silently fails everywhere else.
//
// So when the operator names no status, this action asks the org which of its
// own statuses actually closes a case. CaseStatus is a queryable standard object
// carrying an IsClosed flag per value, which makes that a single cheap lookup
// rather than guesswork.
package crm_salesforce_case_close

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Close Case"
	Description  = "Close a Salesforce case and record why. Leave Status blank and Flomation uses whichever status your own org treats as closed, so the flow keeps working in an org that renamed it."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+circle-check"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// fallbackClosedStatus is Salesforce's out-of-the-box closed status, used when
// the org cannot tell us its own. It is the right guess — every unmodified org
// has it — and letting Salesforce reject it produces a far more useful error
// than refusing to try.
const fallbackClosedStatus = "Closed"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	{Name: "case_id", Type: core.ConnectionTypeString, Label: "Case ID", Placeholder: "5005f00000XyzAAAAQ — the case to close", Required: true},

	{Name: "case_status", Type: core.ConnectionTypeString, Label: "Closed Status", Placeholder: "Leave blank to use your org's own closed status"},
	{Name: "case_reason", Type: core.ConnectionTypeString, Label: "Reason for Closing", Placeholder: "Instructions not clear, Equipment complexity — must match your org's Case Reason list"},

	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Resolution__c":"Replaced the toner cartridge"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Case ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Fields Written"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	caseID, err := salesforce.RequiredString("case_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("case_id is required — the ID of the case to close")
	}
	if err := salesforce.ValidateRecordID(caseID); err != nil {
		return nil, err
	}

	// An operator-supplied status always wins: they know their org, and an org
	// with several closed statuses has a reason for picking between them.
	status := salesforce.OptionalString("case_status", inputs)
	resolved := false
	if status == "" {
		status = resolveClosedStatus(instanceURL, token)
		resolved = true
	}

	body := map[string]interface{}{"Status": status}
	salesforce.SetIfPresent(body, inputs, "Reason", "case_reason")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	// additional_fields is merged last and therefore WINS, so a Status supplied
	// there overrides both the operator's Closed Status box and anything resolved
	// from the org. Re-read it out of the body rather than trusting the local
	// variable: the "Fields Written" output and the run-log line would otherwise
	// name a status that was never sent, and status_resolved_from_org would claim
	// a lookup decided a value the operator actually typed.
	if override, ok := body["Status"].(string); ok && override != status {
		status = override
		resolved = false
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Case", caseID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// PATCH answers 204 No Content, so the ID we were given and the status we
	// chose are the whole of what there is to hand on — and the status is the
	// interesting half when it was resolved rather than typed.
	result := map[string]interface{}{"Id": caseID, "Status": status, "status_resolved_from_org": resolved}
	summary := fmt.Sprintf("Closed case %s with status %q", caseID, status)
	if resolved {
		summary = fmt.Sprintf("Closed case %s using your org's closed status %q", caseID, status)
	}
	return salesforce.RecordResult(caseID, result, summary), nil
}

// resolveClosedStatus asks the org which of its Case statuses closes a case.
//
// CaseStatus is a standard, queryable object: one row per value of the Status
// picklist, each flagged IsClosed and ordered by SortOrder. Reading it turns
// "type the right word or nothing happens" into a question the org answers for
// itself, which is the entire reason this action exists.
//
// Preference order: the org's DEFAULT closed status, then the first closed
// status by sort order, then Salesforce's stock "Closed". The last of those is
// the fallback for every failure path too — a restricted CaseStatus, an
// unexpected response shape, an org with no closed status configured at all.
// Refusing to close a case because a metadata lookup failed would be worse than
// trying the value that works in almost every org.
//
// ApiName (not MasterLabel) is what the Status field stores: MasterLabel is the
// display label, and they diverge the moment an admin renames a value — which
// is precisely the org this lookup exists for.
func resolveClosedStatus(instanceURL, token string) string {
	soql, err := salesforce.BuildQuery(
		"CaseStatus",
		"MasterLabel,ApiName,IsDefault,IsClosed",
		[]salesforce.Condition{{Field: "IsClosed", Operator: "=", Value: "true"}},
		false,
		"SortOrder",
		0,
		false,
	)
	if err != nil {
		log.WithError(err).Warn("Salesforce close case: could not build the closed-status lookup; falling back to \"Closed\"")
		return fallbackClosedStatus
	}

	records, _, _, _, err := salesforce.Query(instanceURL, token, soql, false, false)
	if err != nil {
		log.WithError(err).Warn("Salesforce close case: could not read the org's closed statuses; falling back to \"Closed\"")
		return fallbackClosedStatus
	}

	first := ""
	for _, record := range records {
		value := statusValue(record)
		if value == "" {
			continue
		}
		if isDefault, _ := record["IsDefault"].(bool); isDefault {
			return value
		}
		if first == "" {
			first = value
		}
	}
	if first != "" {
		return first
	}
	return fallbackClosedStatus
}

// statusValue picks the value the Status field actually stores, preferring
// ApiName and falling back to the display label for the rare org where ApiName
// comes back empty.
func statusValue(record map[string]interface{}) string {
	if v, ok := record["ApiName"].(string); ok && v != "" {
		return v
	}
	if v, ok := record["MasterLabel"].(string); ok && v != "" {
		return v
	}
	return ""
}

// Package salesforce_poll declares the "Salesforce record created or updated"
// trigger.
//
// Like every poll trigger in this repo (database_row, s3, git_poll, ...) the
// executor half is purely declarative: Inputs are the poll configuration the
// Launch poller reads, Outputs are the shape Launch populates when it fires, and
// Execute simply echoes any injected data. The polling loop lives in Launch
// (internal/salesforcepoll).
//
// WHY POLLING, when Salesforce has purpose-built change tracking. Salesforce
// exposes /sobjects/{object}/updated/ and /deleted/ for exactly this job, and
// they were the obvious choice — but measured against a live org their coverage
// runs HOURS behind:
//
//	latestDateCovered: 2026-07-26T05:00Z   (queried at 15:03Z — 10.1 hours late)
//
// They are built for batch replication, not for a trigger anyone expects to fire
// promptly, so this polls SOQL instead and tracks its own watermark.
//
// WHY SystemModstamp rather than LastModifiedDate: Salesforce updates
// SystemModstamp for system-level changes that leave LastModifiedDate untouched,
// so a LastModifiedDate cursor silently misses those edits. Salesforce's own
// replication guidance says the same.
//
// DELETES ARE NOT OFFERED. Polling cannot see a deleted record — it is gone from
// the query. /deleted/ exists but carries the same hours-long lag as above, so a
// "record deleted" event would fire long after the fact. Better absent than
// quietly late.
//
// THE COST, and why the default is FIFTEEN MINUTES. One poll is one call against
// the org's daily API allowance, so per trigger per day:
//
//	every 60s   1,440 calls   ~10 triggers exhaust a Developer org (~15,000/day)
//	every 5m      288 calls   ~52 triggers
//	every 15m      96 calls   ~150 triggers
//
// That allowance is shared with everything else the customer does in Salesforce —
// their own integrations, reports, the mobile app — so a default consuming a tenth
// of it per trigger is not a neutral choice. Fifteen minutes keeps a realistic
// number of triggers well inside the budget; anyone who needs it faster can say so
// per trigger, down to the one-minute floor.
package salesforce_poll

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Record Created or Updated"
	Description  = "Triggers a flow when a Salesforce record is created — or created or changed — on any object, including your own custom ones. Checks on an interval and only ever fires for records it has not already seen."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+bolt"
	Date         = "26/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Object To Watch", Placeholder: "Lead, Opportunity, or your own object", Required: true},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Fire When", Required: true, Options: []core.ConnectionOption{
		// The value strings are the cursor FIELD, so Launch needs no mapping table
		// and a new event is a one-line change here.
		{Name: "A record is created", Value: "CreatedDate"},
		{Name: "A record is created or changed", Value: "SystemModstamp"},
	}},
	{Name: "poll_interval", Type: core.ConnectionTypeString, Label: "How Often To Check", Placeholder: "15m by default - every check spends one of your Salesforce API calls, so leave it as long as you can live with"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields To Fetch", Placeholder: "Name, Email, Status - comma separated. Leave blank for the usual ones"},
	{Name: "where", Type: core.ConnectionTypeString, Label: "Only Records Matching", Placeholder: "Status = 'Open' - a SOQL condition, no WHERE (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "record", Type: core.ConnectionTypeObject, Label: "Record"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Object"},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Fired Because"},
	{Name: "cursor", Type: core.ConnectionTypeString, Label: "Cursor Value"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// Execute echoes any injected configuration. Poll-based triggers do no work
// inside the executor — Launch drives the polling loop and populates the outputs.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}
	return result, nil
}

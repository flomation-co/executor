// Package crm_salesforce_record_get_deleted lists the records deleted on an
// object inside a time window, with the time each one went.
//
// Deletes are invisible to every polling approach built on queries: a record
// that has gone simply stops appearing, so a downstream system keeps a copy of
// something the CRM no longer has. This endpoint is the only cheap way to see
// them, and the IDs it returns feed straight into Restore Record if the
// deletion was a mistake.
//
// The window rules match Get Updated Records — at least a minute, end after
// start, start within the last 30 days — and are checked here because
// Salesforce's own answer is a bare INVALID_REPLICATION_DATE.
package crm_salesforce_record_get_deleted

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Deleted Records"
	Description  = "List the records deleted on any Salesforce object between two times, with when each one went. Deletions are invisible to ordinary searches, so this is the only way a flow can keep another system in step."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+xmark"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Salesforce Object", Placeholder: "Account, Contact, Opportunity — or a custom one like Invoice__c", Required: true},
	{Name: "start", Type: core.ConnectionTypeDateTime, Label: "Deleted Since", Placeholder: "2026-07-24T09:00:00Z — must be within the last 30 days", Required: true},
	{Name: "end", Type: core.ConnectionTypeDateTime, Label: "Deleted Before", Placeholder: "Leave blank for right now"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Deleted Records"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Available"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// deletedResponse is Salesforce's getDeleted envelope. earliestDateAvailable
// matters as much as the records: it is how far back the Recycle Bin log
// actually reaches for this org, which is often less than the 30-day maximum.
type deletedResponse struct {
	DeletedRecords []struct {
		ID          string `json:"id"`
		DeletedDate string `json:"deletedDate"`
	} `json:"deletedRecords"`
	EarliestDateAvailable string `json:"earliestDateAvailable"`
	LatestDateCovered     string `json:"latestDateCovered"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	object, err := salesforce.RequiredString("object", inputs)
	if err != nil {
		return nil, err
	}
	object, err = salesforce.ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}

	start, end, err := parseWindow(inputs)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("start", formatWindowTime(start))
	q.Set("end", formatWindowTime(end))
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/"+object+"/deleted?"+q.Encode(), nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	var dr deletedResponse
	if err := json.Unmarshal(resp.Body, &dr); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("could not read the deleted-records list from Salesforce: %v", err)), nil
	}

	// Id is capitalised to match every other record this node emits, so a Loop
	// feeding Restore Record or a downstream delete reads the same field name
	// regardless of which action produced the list.
	records := make([]map[string]interface{}, 0, len(dr.DeletedRecords))
	for _, d := range dr.DeletedRecords {
		records = append(records, map[string]interface{}{
			"Id":          d.ID,
			"deletedDate": d.DeletedDate,
			"attributes":  map[string]interface{}{"type": object},
		})
	}

	summary := fmt.Sprintf("%d %s record(s) deleted between %s and %s", len(records), object, formatWindowTime(start), formatWindowTime(end))
	out := salesforce.ListResult(records, "", len(records), summary)
	// Both dates are operationally useful: latestDateCovered is the cursor for
	// the next run, earliestDateAvailable tells the operator how far back this
	// org's log really goes when the answer comes back empty.
	if result, ok := out["result"].(map[string]interface{}); ok {
		result["latestDateCovered"] = dr.LatestDateCovered
		result["earliestDateAvailable"] = dr.EarliestDateAvailable
	}
	return out, nil
}

// parseWindow reads and sanity-checks the time window. Salesforce rejects a
// window shorter than a minute or reversed, and returns a terse
// INVALID_REPLICATION_DATE for a start older than 30 days — all three are
// configuration mistakes, so they fail hard with an explanation instead.
func parseWindow(inputs []*core.Connection) (time.Time, time.Time, error) {
	startRaw, err := salesforce.RequiredString("start", inputs)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("a start time is required — the point to look for deletions since")
	}
	start, err := parseWindowTime("the start time", startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	end := time.Now().UTC()
	if endRaw := salesforce.OptionalString("end", inputs); endRaw != "" {
		end, err = parseWindowTime("the end time", endRaw)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("the end time must be after the start time — Salesforce was asked for deletions between %s and %s", formatWindowTime(start), formatWindowTime(end))
	}
	if end.Sub(start) < time.Minute {
		return time.Time{}, time.Time{}, fmt.Errorf("the window must cover at least one minute — Salesforce rejects anything shorter")
	}
	if time.Since(start) > 30*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("the start time is more than 30 days ago — Salesforce only keeps the deletion log for 30 days")
	}
	return start, end, nil
}

// windowLayouts are the forms an operator (or an upstream node) plausibly
// supplies a time in. RFC3339 must come first — it is the only one that
// understands a trailing Z or an offset.
var windowLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

func parseWindowTime(label, raw string) (time.Time, error) {
	v := strings.TrimSpace(raw)
	for _, layout := range windowLayouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("%s is not a date Salesforce understands — use 2026-07-24, 2026-07-24 09:00:00 or 2026-07-24T09:00:00Z (got %q)", label, raw)
}

// formatWindowTime renders a time in the offset form Salesforce's replication
// endpoints document. A bare "Z" is accepted by some org configurations and not
// others, so the explicit +00:00 offset is used everywhere.
func formatWindowTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05-07:00")
}

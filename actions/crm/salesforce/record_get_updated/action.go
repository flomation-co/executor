// Package crm_salesforce_record_get_updated lists the IDs of records changed on
// an object inside a time window.
//
// It is the cheap, correct substrate for change detection. The alternative a
// no-code user reaches for — query the whole object every run and compare — is
// hundreds of API calls and misses nothing except the thing that matters. This
// is ONE call that returns exactly what moved, plus latestDateCovered: the
// timestamp Salesforce guarantees it has covered up to, which is the value to
// feed back in as the next run's start so nothing falls through the gap.
//
// Salesforce's own constraints on the window are not obvious and produce blunt
// errors, so they are checked here: the window must be at least a minute long,
// end must follow start, and start must be within the last 30 days.
package crm_salesforce_record_get_updated

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
	Name         = "Salesforce: Get Updated Records"
	Description  = "List the records on any Salesforce object that were created or changed between two times. One cheap call instead of re-reading the whole object, so a scheduled flow can act on just what moved."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clock-rotate-left"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Salesforce Object", Placeholder: "Account, Contact, Opportunity — or a custom one like Invoice__c", Required: true},
	{Name: "start", Type: core.ConnectionTypeDateTime, Label: "Changed Since", Placeholder: "2026-07-24T09:00:00Z — must be within the last 30 days", Required: true},
	{Name: "end", Type: core.ConnectionTypeDateTime, Label: "Changed Before", Placeholder: "Leave blank for right now"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Changed Records"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Available"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// updatedResponse is Salesforce's getUpdated envelope: a flat list of IDs plus
// the high-water mark the caller should resume from.
type updatedResponse struct {
	IDs               []string `json:"ids"`
	LatestDateCovered string   `json:"latestDateCovered"`
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
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/"+object+"/updated?"+q.Encode(), nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	var ur updatedResponse
	if err := json.Unmarshal(resp.Body, &ur); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("could not read the changed-records list from Salesforce: %v", err)), nil
	}

	// Shape each ID as a record so a Loop can hand it straight to Get Record or
	// Update Record. attributes.type is the shape Salesforce itself uses, which
	// keeps the output consistent with every other list action in the node.
	records := make([]map[string]interface{}, 0, len(ur.IDs))
	for _, id := range ur.IDs {
		records = append(records, map[string]interface{}{
			"Id":         id,
			"attributes": map[string]interface{}{"type": object},
		})
	}

	summary := fmt.Sprintf("%d %s record(s) changed between %s and %s", len(records), object, formatWindowTime(start), formatWindowTime(end))
	out := salesforce.ListResult(records, "", len(records), summary)
	// latestDateCovered is the cursor for the next run — Salesforce guarantees
	// it has covered changes up to this point, which is NOT always the end time
	// asked for. Surface it on the raw result so a scheduled flow can store it.
	if result, ok := out["result"].(map[string]interface{}); ok {
		result["latestDateCovered"] = ur.LatestDateCovered
		result["ids"] = ur.IDs
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
		return time.Time{}, time.Time{}, fmt.Errorf("a start time is required — the point to look for changes since")
	}
	start, err := parseWindowTime("the start time", startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	// Default the end to now: "everything since my last run" is what a
	// scheduled flow wants, and asking the operator to supply "now" is busywork.
	end := time.Now().UTC()
	if endRaw := salesforce.OptionalString("end", inputs); endRaw != "" {
		end, err = parseWindowTime("the end time", endRaw)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("the end time must be after the start time — Salesforce was asked for changes between %s and %s", formatWindowTime(start), formatWindowTime(end))
	}
	if end.Sub(start) < time.Minute {
		return time.Time{}, time.Time{}, fmt.Errorf("the window must cover at least one minute — Salesforce rejects anything shorter")
	}
	if time.Since(start) > 30*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("the start time is more than 30 days ago — Salesforce only keeps this change log for 30 days. Use Get Many Records with a LastModifiedDate filter for anything older")
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

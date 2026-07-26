// Package crm_salesforce_report_run runs a saved Salesforce report and returns
// its numbers.
//
// This is the cheapest possible route to "every Monday, run the pipeline report
// and post the total in Slack": the admin already built the report, so the
// operator needs no query skills at all — they pick a report and read the
// totals off the result. Filters can be overridden per run without touching the
// saved report, which is what makes one report serve a dozen automations.
package crm_salesforce_report_run

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Run Report"
	Description  = "Run one of your saved Salesforce reports and get its totals back, optionally overriding the report's filters just for this run."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+chart-bar"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "report_id", Type: core.ConnectionTypeString, Label: "Report", Placeholder: "00O5f000004XyzAEAS — the report ID from Get Many Reports", Required: true},
	{Name: "include_details", Type: core.ConnectionTypeBoolean, Label: "Include the Individual Rows"},
	{Name: "report_filters", Type: core.ConnectionTypeObject, Label: "Filters for This Run (JSON)", Placeholder: `[{"column":"CREATED_DATE","operator":"greaterThan","value":"THIS_MONTH"}]`},
	{Name: "filter_logic", Type: core.ConnectionTypeString, Label: "Filter Logic", Placeholder: "1 AND (2 OR 3) — only needed with two or more filters"},
	{Name: "report_metadata", Type: core.ConnectionTypeObject, Label: "Advanced Report Settings (JSON)", Placeholder: `{"standardDateFilter":{"column":"CLOSE_DATE","durationValue":"THIS_QUARTER"}}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Report ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Report Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// grandTotalKey addresses the "everything" cell of a report's fact map — the
// row that holds the grand totals, whatever grouping the report uses. Summary
// and matrix reports fill the rest of the map with per-grouping cells; this key
// is the only one guaranteed to exist on every report format.
const grandTotalKey = "T!T"

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	reportID, err := salesforce.RequiredString("report_id", inputs)
	if err != nil {
		return nil, err
	}
	if err := salesforce.ValidateRecordID(reportID); err != nil {
		return nil, err
	}

	metadata, err := buildReportMetadata(inputs)
	if err != nil {
		return nil, err
	}

	path := "/analytics/reports/" + url.PathEscape(reportID)
	includeDetails := salesforce.OptionalBool("include_details", inputs)
	if includeDetails {
		// Off by default because the row-level detail is capped at 2,000 rows
		// and is usually far larger than the totals the flow actually wants.
		path += "?includeDetails=true"
	}

	// A plain run is a GET; overrides go in the body of a POST. Sending the
	// POST unconditionally would mean posting an empty reportMetadata on every
	// ordinary run, which is a needless way to find out how Salesforce treats
	// an empty override object.
	method := http.MethodGet
	var body interface{}
	if len(metadata) > 0 {
		method = http.MethodPost
		body = map[string]interface{}{"reportMetadata": metadata}
	}

	resp, err := salesforce.ExecuteAPI(instanceURL, token, method, path, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse the Salesforce report result: %v", err)), nil
	}

	return salesforce.RecordResult(reportID, payload, summarise(payload, reportID, includeDetails)), nil
}

// buildReportMetadata assembles the per-run overrides.
//
// Salesforce merges whatever properties are supplied over the report's saved
// definition, so a partial object is exactly right: sending only reportFilters
// changes the filters and leaves the columns, groupings and sort untouched. The
// advanced JSON is applied first so the friendly inputs win on a clash.
func buildReportMetadata(inputs []*core.Connection) (map[string]interface{}, error) {
	metadata := map[string]interface{}{}
	if err := salesforce.MergeJSONObject(metadata, inputs, "report_metadata"); err != nil {
		return nil, err
	}

	filters, err := parseFilters(inputs)
	if err != nil {
		return nil, err
	}
	if filters != nil {
		metadata["reportFilters"] = filters
	}
	if logic := salesforce.OptionalString("filter_logic", inputs); logic != "" {
		metadata["reportBooleanFilter"] = logic
	}
	return metadata, nil
}

// parseFilters reads the filter-override input, which Salesforce expects as an
// array of {column, operator, value}. Returns nil when unset.
func parseFilters(inputs []*core.Connection) ([]interface{}, error) {
	v, err := salesforce.OptionalJSON("report_filters", inputs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf(`filters must be a JSON array, e.g. [{"column":"CREATED_DATE","operator":"greaterThan","value":"THIS_MONTH"}]`)
	}
	for i, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("filters[%d] must be an object with column, operator and value", i)
		}
		// The column is the report's own API column name (CREATED_DATE,
		// OPPORTUNITY.AMOUNT). Checking it is present here turns a Salesforce
		// 400 nobody can read into a message that names the offending filter.
		if column, _ := obj["column"].(string); strings.TrimSpace(column) == "" {
			return nil, fmt.Errorf(`filters[%d] is missing "column" — use the report column's API name, e.g. CREATED_DATE`, i)
		}
	}
	return arr, nil
}

// summarise turns the report payload into one plain-English line: the report's
// name and its grand totals, which is the part an operator wants to read in a
// Slack message or an execution log.
func summarise(payload map[string]interface{}, reportID string, includeDetails bool) string {
	name := reportName(payload)
	if name == "" {
		name = reportID
	}

	parts := []string{fmt.Sprintf("Ran the %s report", name)}
	if totals := grandTotals(payload); totals != "" {
		parts = append(parts, totals)
	}
	if includeDetails {
		parts = append(parts, fmt.Sprintf("%d row(s) returned", detailRowCount(payload)))
	}
	// allData=false means Salesforce truncated the result — the synchronous run
	// stops at 2,000 detail rows. Saying so is the difference between a total
	// that is wrong and a total the operator knows to distrust.
	if allData, ok := payload["allData"].(bool); ok && !allData {
		parts = append(parts, "results were truncated by Salesforce's 2,000-row limit on an instant run — add filters to narrow the report")
	}
	return strings.Join(parts, " — ")
}

// reportName digs the report's name out of the response. It lives in two
// places depending on the report format, so both are tried.
func reportName(payload map[string]interface{}) string {
	if meta, ok := payload["reportMetadata"].(map[string]interface{}); ok {
		if name, _ := meta["name"].(string); name != "" {
			return name
		}
	}
	if attrs, ok := payload["attributes"].(map[string]interface{}); ok {
		if name, _ := attrs["reportName"].(string); name != "" {
			return name
		}
	}
	return ""
}

// grandTotals renders the report's totals, pairing each aggregate value in the
// fact map with the name of the aggregate it belongs to. The two lists are
// positional — reportMetadata.aggregates[i] names factMap["T!T"].aggregates[i]
// — and nothing in the response links them any other way.
func grandTotals(payload map[string]interface{}) string {
	factMap, ok := payload["factMap"].(map[string]interface{})
	if !ok {
		return ""
	}
	cell, ok := factMap[grandTotalKey].(map[string]interface{})
	if !ok {
		return ""
	}
	values, ok := cell["aggregates"].([]interface{})
	if !ok || len(values) == 0 {
		return ""
	}

	var names []interface{}
	if meta, ok := payload["reportMetadata"].(map[string]interface{}); ok {
		names, _ = meta["aggregates"].([]interface{})
	}

	parts := make([]string, 0, len(values))
	for i, v := range values {
		agg, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		// label is Salesforce's own formatted rendering ("£1,250.00", "57"),
		// which is what an operator should see; value is the raw number and
		// stays available in the result output for anything that must compute.
		text, _ := agg["label"].(string)
		if text == "" {
			text = fmt.Sprintf("%v", agg["value"])
		}
		label := "Total"
		if i < len(names) {
			if code, _ := names[i].(string); code != "" {
				label = aggregateLabel(code)
			}
		}
		parts = append(parts, label+": "+text)
	}
	return strings.Join(parts, ", ")
}

// aggregateLabel turns a Salesforce aggregate code into plain English.
// "s!Amount" and "RowCount" mean nothing to a receptionist; "Sum of Amount" and
// "Record count" do.
func aggregateLabel(code string) string {
	if strings.EqualFold(code, "RowCount") {
		return "Record count"
	}
	i := strings.Index(code, "!")
	if i < 0 {
		return code
	}
	field := code[i+1:]
	switch strings.ToLower(code[:i]) {
	case "s":
		return "Sum of " + field
	case "a":
		return "Average " + field
	case "mx":
		return "Largest " + field
	case "mn":
		return "Smallest " + field
	case "u":
		return "Unique " + field
	}
	return field
}

// detailRowCount counts the individual rows behind the grand total, which are
// only present when the run asked for them.
func detailRowCount(payload map[string]interface{}) int {
	factMap, ok := payload["factMap"].(map[string]interface{})
	if !ok {
		return 0
	}
	cell, ok := factMap[grandTotalKey].(map[string]interface{})
	if !ok {
		return 0
	}
	rows, _ := cell["rows"].([]interface{})
	return len(rows)
}

// Package crm_salesforce_report_get_all lists the saved reports in the org, so
// "Run Report" can be pointed at one the admin has already built rather than
// anyone having to write a query.
package crm_salesforce_report_get_all

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Reports"
	Description  = "List the saved Salesforce reports you can run, with the ID that Run Report needs. Search by name or narrow it to one report folder."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{
		Name:        "scope",
		Type:        core.ConnectionTypeString,
		Label:       "Which Reports",
		Placeholder: "Every report you can see",
		Options: []core.ConnectionOption{
			{Name: "Every report you can see", Value: "all"},
			{Name: "Only the ones you opened recently", Value: "recent"},
		},
	},
	{Name: "name_contains", Type: core.ConnectionTypeString, Label: "Name Contains", Placeholder: "pipeline — leave blank to list them all"},
	{Name: "folder", Type: core.ConnectionTypeString, Label: "Report Folder", Placeholder: "Sales Reports — leave blank to search every folder"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 reports (max 2000); ignored when Return All is on"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Reports"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Available"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// reportFields is the SELECT list for the query path. DeveloperName and
// FolderName are included because two reports in different folders very often
// share a name, and the folder is the only thing that tells them apart when an
// operator is picking one.
const reportFields = "Id,Name,DeveloperName,FolderName,Format,LastRunDate,LastModifiedDate"

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// Two very different sources, and the default matters. Salesforce's
	// analytics endpoint only returns the reports the CONNECTED user has opened
	// recently — for the service account a flow usually runs as, that list is
	// frequently empty, which makes it useless as the thing you pick a report
	// from. Querying the Report object instead returns every report the user
	// can see, in every folder, so that is the default; "recent" is kept
	// because it is the only way to get Salesforce's own recency ordering.
	if strings.EqualFold(salesforce.OptionalString("scope", inputs), "recent") {
		return recentlyViewed(instanceURL, token, inputs)
	}
	return allReports(instanceURL, token, inputs)
}

// allReports lists every report the connected user can see by querying the
// Report object, which supports proper server-side filtering and pagination.
func allReports(instanceURL, token string, inputs []*core.Connection) (map[string]interface{}, error) {
	var conditions []salesforce.Condition
	if name := salesforce.OptionalString("name_contains", inputs); name != "" {
		// LIKE with wildcards either side is the "contains" an operator means
		// when they type a word into a search box. The value is escaped and
		// quoted by the shared SOQL builder, never concatenated here.
		conditions = append(conditions, salesforce.Condition{Field: "Name", Operator: "LIKE", Value: "%" + name + "%"})
	}
	if folder := salesforce.OptionalString("folder", inputs); folder != "" {
		conditions = append(conditions, salesforce.Condition{Field: "FolderName", Operator: "=", Value: folder})
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	pageSize := salesforce.ClampLimit(limit, limitSet)

	soql, err := salesforce.BuildQueryTyped(instanceURL, token, "Report", reportFields, conditions, false, "Name", pageSize, !returnAll)
	if err != nil {
		return nil, err
	}

	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, false)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Found %d report(s)", len(records))
	if returnAll {
		summary = fmt.Sprintf("Fetched all %d report(s) across %d page(s)", len(records), pages)
		if nextURL != "" && pages >= salesforce.MaxAllPages {
			summary = fmt.Sprintf("Fetched %d report(s) across %d page(s); stopped at the %d-page safety cap — narrow the search to see the rest", len(records), pages, salesforce.MaxAllPages)
		}
	}
	return salesforce.ListResult(records, nextURL, totalSize, summary), nil
}

// recentlyViewed lists the reports the connected user has opened recently, via
// the Analytics API. Salesforce caps this at 200 and does not paginate it, so
// the search and the limit are applied client-side and next_url stays empty.
func recentlyViewed(instanceURL, token string, inputs []*core.Connection) (map[string]interface{}, error) {
	// Each entry on this path carries only {id, name, url} — no folder — so a
	// folder filter cannot be applied to it at all. Ignoring it silently would
	// be the worst outcome available: the operator believes they have narrowed
	// the list to one folder and the flow loops over reports from every folder
	// in the org. Say which setting to change instead.
	if folder := salesforce.OptionalString("folder", inputs); folder != "" {
		return nil, fmt.Errorf("the Report Folder filter cannot be used with \"Only the ones you opened recently\" — Salesforce's recently-viewed list does not say which folder a report is in. Set Which Reports to \"Every report you can see\" to filter by folder, or clear the folder")
	}

	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/analytics/reports", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// The Analytics list endpoint answers with a bare JSON array, not the
	// {records: [...]} envelope the query endpoints use.
	var reports []map[string]interface{}
	if err := json.Unmarshal(resp.Body, &reports); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse the Salesforce report list: %v", err)), nil
	}

	matched := filterByName(reports, salesforce.OptionalString("name_contains", inputs))
	total := len(matched)

	if !salesforce.OptionalBool("return_all", inputs) {
		if limit, set := salesforce.OptionalInt("limit", inputs); set && limit > 0 && limit < len(matched) {
			matched = matched[:limit]
		}
	}

	summary := fmt.Sprintf("Found %d recently viewed report(s)", len(matched))
	if len(matched) < total {
		summary = fmt.Sprintf("Showing %d of %d recently viewed report(s)", len(matched), total)
	}
	if total == 0 {
		summary = "No recently viewed reports — Salesforce only lists reports this connected user has opened, so switch Which Reports to \"Every report you can see\""
	}
	return salesforce.ListResult(matched, "", total, summary), nil
}

// filterByName keeps the reports whose name contains the operator's search
// text, case-insensitively. The Analytics list has no server-side search, so
// this is the only filter available on that path.
func filterByName(reports []map[string]interface{}, contains string) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(reports))
	needle := strings.ToLower(strings.TrimSpace(contains))
	for _, r := range reports {
		if needle == "" {
			out = append(out, r)
			continue
		}
		name, _ := r["name"].(string)
		if strings.Contains(strings.ToLower(name), needle) {
			out = append(out, r)
		}
	}
	return out
}

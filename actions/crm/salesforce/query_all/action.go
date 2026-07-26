package crm_salesforce_query_all

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Run Query Including Deleted"
	Description  = "Run a Salesforce query that also returns records in the Recycle Bin and archived activities — the records people think have vanished. Select IsDeleted in your query to see which ones are deleted, or add WHERE IsDeleted = true to see only those."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clock-rotate-left"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "query", Type: core.ConnectionTypeText, Label: "Query", Placeholder: "SELECT Id, Name, IsDeleted FROM Account WHERE IsDeleted = true", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return Every Page", Placeholder: "Keep fetching until every matching record has been returned"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Records"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Matching Records"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	soql, err := salesforce.RequiredString("query", inputs)
	if err != nil {
		return nil, err
	}
	// The operator's SOQL is sent exactly as written — this action deliberately
	// does not rewrite it (an injected IsDeleted term would have to be spliced
	// ahead of ORDER BY / LIMIT and could contradict a filter they already
	// wrote). The checks below only catch a statement that is not a query at
	// all, which Salesforce would otherwise answer with an opaque
	// MALFORMED_QUERY after a wasted call against the org's API allowance.
	if err := validateStatement(soql); err != nil {
		return nil, err
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)

	// includeDeleted selects /queryAll rather than /query — the only difference
	// between this action and Run Query, and the only way to see the Recycle
	// Bin and archived activities that /query silently hides.
	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, true)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	out["tool_result"] = summarise(records, totalSize, pages, nextURL, returnAll)
	return out, nil
}

// summarise renders the plain-English result line, adding how many of the
// returned records are actually deleted — but only when the operator selected
// IsDeleted, because otherwise the field is absent from the response and a
// count of zero would be a guess rather than a fact.
func summarise(records []map[string]interface{}, totalSize, pages int, nextURL string, returnAll bool) string {
	count := len(records)

	var base string
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		base = fmt.Sprintf("Returned %d record(s) across %d page(s), then stopped at the %d-page safety limit — narrow the query with a WHERE clause or a LIMIT", count, pages, salesforce.MaxAllPages)
	case returnAll:
		base = fmt.Sprintf("Returned all %d record(s) across %d page(s), including deleted and archived", count, pages)
	case nextURL != "":
		base = fmt.Sprintf("Returned %d of %d matching record(s), including deleted and archived — turn on Return Every Page to fetch the rest", count, totalSize)
	default:
		base = fmt.Sprintf("Returned %d record(s), including deleted and archived", count)
	}

	if deleted, reported := countDeleted(records); reported {
		base += fmt.Sprintf(" — %d of them are in the Recycle Bin", deleted)
	}
	return base
}

// countDeleted tallies the records Salesforce flagged as deleted. The bool is
// false when IsDeleted was not part of the SELECT list, so the caller stays
// quiet rather than reporting a zero it cannot vouch for.
func countDeleted(records []map[string]interface{}) (int, bool) {
	deleted := 0
	present := false
	for _, r := range records {
		v, ok := r["IsDeleted"]
		if !ok {
			continue
		}
		present = true
		// Salesforce sends a real JSON boolean here, but a value reached through
		// a relationship can arrive as a string; accept both rather than
		// under-reporting.
		switch tv := v.(type) {
		case bool:
			if tv {
				deleted++
			}
		case string:
			if strings.EqualFold(tv, "true") {
				deleted++
			}
		}
	}
	return deleted, present
}

// validateStatement mirrors the guard on the plain Run Query action: catch a
// SOSL search or a non-query before it costs a round trip.
func validateStatement(soql string) error {
	trimmed := strings.TrimSpace(soql)
	// Collapse newlines and runs of spaces for the CHECK only — a multi-line
	// query is perfectly valid and is sent exactly as the operator wrote it.
	normalised := strings.ToUpper(strings.Join(strings.Fields(trimmed), " "))

	if strings.HasPrefix(normalised, "FIND") || strings.HasPrefix(normalised, "{") {
		return fmt.Errorf("that looks like a SOSL search rather than a SOQL query — use the Salesforce: Find Records action for a plain-text search across objects")
	}
	if !strings.HasPrefix(normalised, "SELECT") {
		return fmt.Errorf("a Salesforce query must start with SELECT, for example: SELECT Id, Name, IsDeleted FROM Account — got %q", truncate(trimmed, 60))
	}
	if !strings.Contains(normalised, " FROM ") {
		return fmt.Errorf("a Salesforce query needs a FROM clause naming the object to search, for example: SELECT Id, Name FROM Account — got %q", truncate(trimmed, 60))
	}
	return nil
}

// truncate shortens a quoted fragment so an error message about a long query is
// still readable in the execution view.
func truncate(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

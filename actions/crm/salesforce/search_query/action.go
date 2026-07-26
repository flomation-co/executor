package crm_salesforce_search_query

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Run Query (SOQL)"
	Description  = "Run a Salesforce query you have written yourself and return the matching records. This is the advanced, power-user path — if you would rather not write a query, use Find Records or Find Record by Field instead."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+bolt"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "query", Type: core.ConnectionTypeText, Label: "Query", Placeholder: "SELECT Id, Name, Email FROM Contact WHERE CreatedDate = LAST_N_DAYS:7", Required: true},
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
	// The whole point of this action is that the operator's SOQL reaches
	// Salesforce untouched, so there is no builder to validate against — the
	// query is deliberately NOT rewritten. The two shape checks below are the
	// only guards, and they exist because both mistakes are common and both
	// otherwise come back as an opaque MALFORMED_QUERY from Salesforce after a
	// wasted round trip against the org's daily API allowance.
	if err := ValidateStatement(soql); err != nil {
		return nil, err
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)

	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, false)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	out["tool_result"] = summarise(len(records), totalSize, pages, nextURL, returnAll)
	return out, nil
}

// summarise renders the plain-English result line. It calls out the two states
// an operator most often misreads: a single page that has more behind it (they
// think the query is wrong when it is just unpaged), and hitting the page cap.
func summarise(count, totalSize, pages int, nextURL string, returnAll bool) string {
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		return fmt.Sprintf("Returned %d record(s) across %d page(s), then stopped at the %d-page safety limit — narrow the query with a WHERE clause or a LIMIT", count, pages, salesforce.MaxAllPages)
	case returnAll:
		return fmt.Sprintf("Returned all %d record(s) across %d page(s)", count, pages)
	case nextURL != "":
		return fmt.Sprintf("Returned %d of %d matching record(s) — turn on Return Every Page to fetch the rest", count, totalSize)
	default:
		return fmt.Sprintf("Returned %d record(s)", count)
	}
}

// ValidateStatement rejects the two things operators actually paste into this
// box by mistake: a SOSL search (which belongs in Find Records) and something
// that is not a query at all. Salesforce answers both with MALFORMED_QUERY,
// which tells the person reading it nothing about what to do next.
//
// Exported so a future test package can exercise it without reaching into
// Execute; it is a pure string check with no Salesforce call behind it.
func ValidateStatement(soql string) error {
	trimmed := strings.TrimSpace(soql)
	// Collapse newlines and runs of spaces for the CHECK only — a multi-line
	// query is perfectly valid and is sent exactly as the operator wrote it.
	normalised := strings.ToUpper(strings.Join(strings.Fields(trimmed), " "))

	if strings.HasPrefix(normalised, "FIND") || strings.HasPrefix(normalised, "{") {
		return fmt.Errorf("that looks like a SOSL search rather than a SOQL query — use the Salesforce: Find Records action for a plain-text search across objects")
	}
	if !strings.HasPrefix(normalised, "SELECT") {
		return fmt.Errorf("a Salesforce query must start with SELECT, for example: SELECT Id, Name FROM Account WHERE Industry = 'Retail' — got %q", truncate(trimmed, 60))
	}
	if !strings.Contains(normalised, " FROM ") {
		return fmt.Errorf("a Salesforce query needs a FROM clause naming the object to search, for example: SELECT Id, Name FROM Account — got %q", truncate(trimmed, 60))
	}
	return nil
}

// truncate shortens a quoted fragment so an error message about a 400-line
// query is still readable in the execution view.
func truncate(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

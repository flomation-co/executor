package salesforce

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// describeServer answers the one describe call BuildScopedQueryTyped makes, so
// the field-type-aware path is the one under test rather than the heuristic
// fallback. types is field name -> Salesforce type.
func describeServer(t *testing.T, object string, types map[string]string) func() {
	t.Helper()
	fields := make([]map[string]interface{}, 0, len(types))
	for name, sfType := range types {
		fields = append(fields, map[string]interface{}{"name": name, "type": sfType})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/sobjects/"+object+"/describe") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"name": object, "fields": fields})
	}))
	restore := SetHostForTest(srv.URL)
	return func() {
		restore()
		srv.Close()
	}
}

// TestBuildScopedQueryTypedKeepsScopeANDed is the regression test for the
// "Match ANY filter" bug: the toggle is about the operator's own filters, and
// must never re-join the action's built-in scope terms. Before the split this
// produced `WHERE IsActive = true OR Department = 'Sales' OR ...`, which
// returns every active user in the org PLUS the deactivated ones the tick box
// exists to hide.
func TestBuildScopedQueryTypedKeepsScopeANDed(t *testing.T) {
	defer describeServer(t, "User", map[string]string{
		"isactive":   "boolean",
		"department": "string",
	})()

	scope := []Condition{{Field: "IsActive", Operator: "=", Value: "true"}}
	filters := []Condition{
		{Field: "Department", Operator: "=", Value: "Sales"},
		{Field: "Department", Operator: "=", Value: "Support"},
	}

	got, err := BuildScopedQueryTyped("https://x.my.salesforce.com", "tok", "User", "Id", scope, filters, true, "", 50, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "SELECT Id FROM User WHERE IsActive = true AND (Department = 'Sales' OR Department = 'Support') LIMIT 50"
	if got != want {
		t.Errorf("scope was not kept out of the OR group:\n got %q\nwant %q", got, want)
	}
	if strings.Contains(got, "IsActive = true OR") {
		t.Errorf("the scope term is ORed with the operator's filters: %q", got)
	}
}

// TestBuildScopedQueryTypedRangeStaysANDed covers the destructive shape: two
// ends of a date range ORed together are satisfied by every row that has a date
// at all (and, in SOQL, by rows where it is NULL), so the range silently became
// "everything" and annihilated every other filter on the form.
func TestBuildScopedQueryTypedRangeStaysANDed(t *testing.T) {
	defer describeServer(t, "Task", map[string]string{
		"activitydate": "date",
		"status":       "picklist",
		"subject":      "string",
	})()

	scope := []Condition{
		{Field: "ActivityDate", Operator: ">=", Value: "2026-07-01"},
		{Field: "ActivityDate", Operator: "<=", Value: "2026-07-31"},
	}
	filters := []Condition{
		{Field: "Status", Operator: "=", Value: "Not Started"},
		{Field: "Subject", Operator: "LIKE", Value: "%renewal%"},
	}

	got, err := BuildScopedQueryTyped("https://x.my.salesforce.com", "tok", "Task", "Id", scope, filters, true, "ActivityDate ASC", 50, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "SELECT Id FROM Task " +
		"WHERE ActivityDate >= 2026-07-01 AND ActivityDate <= 2026-07-31 " +
		"AND (Status = 'Not Started' OR Subject LIKE '%renewal%') " +
		"ORDER BY ActivityDate ASC LIMIT 50"
	if got != want {
		t.Errorf("date range was not kept out of the OR group:\n got %q\nwant %q", got, want)
	}
}

// A range with no filters at all must still be ANDed — with the toggle on and
// one flat list this was the pure tautology form.
func TestBuildScopedQueryTypedRangeAloneIgnoresTheToggle(t *testing.T) {
	defer describeServer(t, "Task", map[string]string{"activitydate": "date"})()

	scope := []Condition{
		{Field: "ActivityDate", Operator: ">=", Value: "2026-07-01"},
		{Field: "ActivityDate", Operator: "<=", Value: "2026-07-31"},
	}

	got, err := BuildScopedQueryTyped("https://x.my.salesforce.com", "tok", "Task", "Id", scope, nil, true, "", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "SELECT Id FROM Task WHERE ActivityDate >= 2026-07-01 AND ActivityDate <= 2026-07-31"
	if got != want {
		t.Errorf("range alone must stay ANDed:\n got %q\nwant %q", got, want)
	}
}

// With the toggle off, or with nothing to protect, the emitted SOQL must be
// exactly what the flat builder has always produced — no stray brackets.
func TestBuildScopedQueryTypedMatchesFlatBuilderWhenGroupingChangesNothing(t *testing.T) {
	defer describeServer(t, "User", map[string]string{
		"isactive":   "boolean",
		"department": "string",
	})()

	scope := []Condition{{Field: "IsActive", Operator: "=", Value: "true"}}
	filters := []Condition{{Field: "Department", Operator: "=", Value: "Sales"}}

	all := append(append([]Condition{}, scope...), filters...)
	flat, err := BuildQueryTyped("https://x.my.salesforce.com", "tok", "User", "Id", all, false, "", 50, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	scoped, err := BuildScopedQueryTyped("https://x.my.salesforce.com", "tok", "User", "Id", scope, filters, false, "", 50, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scoped != flat {
		t.Errorf("ALL mode changed shape:\n got %q\nwant %q", scoped, flat)
	}

	// No scope at all: the toggle owns the whole clause, as before.
	onlyFilters, err := BuildScopedQueryTyped("https://x.my.salesforce.com", "tok", "User", "Id", nil, filters, true, "", 50, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "SELECT Id FROM User WHERE Department = 'Sales' LIMIT 50"; onlyFilters != want {
		t.Errorf("filters-only:\n got %q\nwant %q", onlyFilters, want)
	}
}

// NOT LIKE renders as its own bracketed group, and SOQL only parses that form
// when it is bracketed. Nesting it inside the ANY group is verified live:
// `A AND ((NOT (Name LIKE 'z%')) OR B)` is accepted.
func TestBuildScopedQueryTypedNotLikeInsideTheAnyGroup(t *testing.T) {
	defer describeServer(t, "User", map[string]string{
		"isactive":   "boolean",
		"name":       "string",
		"department": "string",
	})()

	scope := []Condition{{Field: "IsActive", Operator: "=", Value: "true"}}
	filters := []Condition{
		{Field: "Name", Operator: "NOT LIKE", Value: "z%"},
		{Field: "Department", Operator: "=", Value: "Sales"},
	}

	got, err := BuildScopedQueryTyped("https://x.my.salesforce.com", "tok", "User", "Id", scope, filters, true, "", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "SELECT Id FROM User WHERE IsActive = true AND ((NOT (Name LIKE 'z%')) OR Department = 'Sales')"
	if got != want {
		t.Errorf("NOT LIKE inside the ANY group:\n got %q\nwant %q", got, want)
	}
}

// Validation must still bite on both lists — the grouping path is not a way
// round the identifier whitelist.
func TestBuildScopedQueryTypedStillValidates(t *testing.T) {
	defer describeServer(t, "User", map[string]string{"isactive": "boolean"})()

	scope := []Condition{{Field: "IsActive", Operator: "=", Value: "true"}}
	bad := []Condition{{Field: "Name; DROP", Operator: "=", Value: "x"}}
	if _, err := BuildScopedQueryTyped("https://x.my.salesforce.com", "tok", "User", "Id", scope, bad, true, "", 0, false); err == nil {
		t.Error("an invalid filter field was accepted")
	}

	badScope := []Condition{{Field: "IsActive", Operator: "sneaky", Value: "true"}}
	ok := []Condition{{Field: "Name", Operator: "=", Value: "x"}, {Field: "Name", Operator: "=", Value: "y"}}
	if _, err := BuildScopedQueryTyped("https://x.my.salesforce.com", "tok", "User", "Id", badScope, ok, true, "", 0, false); err == nil {
		t.Error("an invalid scope operator was accepted")
	}
}

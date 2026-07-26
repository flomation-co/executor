package crm_salesforce_task_get_all

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// queryRecorder stands in for the org: it answers the describe BuildScopedQueryTyped
// makes and records the SOQL the action actually sent.
type queryRecorder struct {
	soql string
}

func (q *queryRecorder) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/sobjects/Task/describe"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "Task",
				"fields": []map[string]interface{}{
					{"name": "ActivityDate", "type": "date"},
					{"name": "Status", "type": "picklist"},
					{"name": "Subject", "type": "string"},
					{"name": "Priority", "type": "picklist"},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/query"):
			q.soql = r.URL.Query().Get("q")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 0, "done": true, "records": []interface{}{},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	restore := salesforce.SetHostForTest(srv.URL)
	return func() {
		restore()
		srv.Close()
	}
}

func auth() []*core.Connection {
	return []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
	}
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// TestMatchAnyFilterDoesNotOrTheDueDateRange is the regression test. With the
// toggle on, the two ends of the range used to be ORed like any other term,
// making the WHERE clause a tautology: every task in the org came back (NULL
// due dates included) and every other filter was annihilated. Live: 6 rows
// ANDed, 33 — the whole table — ORed, with success reported either way.
func TestMatchAnyFilterDoesNotOrTheDueDateRange(t *testing.T) {
	rec := &queryRecorder{}
	defer rec.serve(t)()

	inputs := append(auth(),
		str("due_from", "2026-07-01"),
		str("due_to", "2026-07-31"),
		str("task_status", "Not Started"),
		str("filter_field", "Subject"),
		str("filter_operator", "LIKE"),
		str("filter_value", "%renewal%"),
		&core.Connection{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Value: true},
	)

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}

	if !strings.Contains(rec.soql, "WHERE ActivityDate >= 2026-07-01 AND ActivityDate <= 2026-07-31 AND (") {
		t.Errorf("the due-date range must stay ANDed and the filters bracketed, got:\n%s", rec.soql)
	}
	if strings.Contains(rec.soql, "ActivityDate >= 2026-07-01 OR") {
		t.Errorf("the range is still being ORed away, got:\n%s", rec.soql)
	}
	if !strings.Contains(rec.soql, "(Subject LIKE '%renewal%' OR Status = 'Not Started')") {
		t.Errorf("the operator's own filters should be the ones ORed, got:\n%s", rec.soql)
	}
}

// With the toggle off nothing changes: one flat ANDed clause, as before.
func TestMatchAllFilterKeepsOneFlatClause(t *testing.T) {
	rec := &queryRecorder{}
	defer rec.serve(t)()

	inputs := append(auth(),
		str("due_from", "2026-07-01"),
		str("due_to", "2026-07-31"),
		str("task_status", "Not Started"),
	)
	if _, err := Execute(&core.Flow{}, &core.Node{}, inputs); err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	want := "WHERE ActivityDate >= 2026-07-01 AND ActivityDate <= 2026-07-31 AND Status = 'Not Started'"
	if !strings.Contains(rec.soql, want) {
		t.Errorf("ALL mode should be unchanged:\n got %s\nwant ...%s...", rec.soql, want)
	}
	if strings.Contains(rec.soql, "(") && !strings.Contains(rec.soql, "'(") {
		t.Errorf("ALL mode should not introduce brackets, got:\n%s", rec.soql)
	}
}

// TestFilterComparisonIsADropdown pins the fix for the free-text comparison
// box. Nine sibling Get Many actions offer a labelled Select; this one made the
// operator type a SOQL token by hand, and typing the word they wanted
// ("contains", or even the "Equals" they read off another action) hard-failed
// with a list of SOQL operators.
func TestFilterComparisonIsADropdown(t *testing.T) {
	var operator *core.Connection
	for i := range Inputs {
		if Inputs[i].Name == "filter_operator" {
			operator = &Inputs[i]
		}
	}
	if operator == nil {
		t.Fatal("filter_operator input is missing")
	}
	if len(operator.Options) == 0 {
		t.Fatal("filter_operator must offer a dropdown, not free text — the editor renders a plain text box when Options is empty")
	}
	byValue := map[string]string{}
	for _, o := range operator.Options {
		byValue[o.Value] = o.Name
	}
	for _, want := range []string{"=", "!=", "<", "<=", ">", ">=", "LIKE", "NOT LIKE", "IN", "NOT IN"} {
		if _, ok := byValue[want]; !ok {
			t.Errorf("comparison %q is not offered", want)
		}
	}
	// Every offered value has to be one the query builder accepts, or the
	// dropdown offers a choice that always fails.
	for value := range byValue {
		if _, err := salesforce.ValidateSOQLOperator(value); err != nil {
			t.Errorf("dropdown offers %q, which the builder rejects: %v", value, err)
		}
	}
	if !strings.Contains(byValue["LIKE"], "%") {
		t.Errorf("the LIKE option must tell the operator about the %% wildcard, got %q", byValue["LIKE"])
	}
}

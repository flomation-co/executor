package crm_salesforce_event_get_all

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

type queryRecorder struct {
	soql string
}

func (q *queryRecorder) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/sobjects/Event/describe"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "Event",
				"fields": []map[string]interface{}{
					{"name": "StartDateTime", "type": "datetime"},
					{"name": "Location", "type": "string"},
					{"name": "IsAllDayEvent", "type": "boolean"},
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

// The diary equivalent of the task range bug: "Starts On or After X OR Starts
// On or Before Y" matches every event ever booked, so a week's calendar lookup
// silently returned the whole org's history and reported success.
func TestMatchAnyFilterDoesNotOrTheDateRange(t *testing.T) {
	rec := &queryRecorder{}
	defer rec.serve(t)()

	inputs := append(auth(),
		str("starts_after", "2026-07-01"),
		str("starts_before", "2026-07-07"),
		str("filter_field", "Location"),
		str("filter_operator", "LIKE"),
		str("filter_value", "%Head office%"),
		&core.Connection{Name: "filter_conditions", Type: core.ConnectionTypeObject, Value: `[{"field":"IsAllDayEvent","operator":"=","value":"false"}]`},
		&core.Connection{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Value: true},
	)

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}

	wantScope := "WHERE StartDateTime >= 2026-07-01T00:00:00Z AND StartDateTime <= 2026-07-07T23:59:59Z AND ("
	if !strings.Contains(rec.soql, wantScope) {
		t.Errorf("the date range must stay ANDed and the filters bracketed, got:\n%s", rec.soql)
	}
	if strings.Contains(rec.soql, "T00:00:00Z OR") {
		t.Errorf("the range is still being ORed away, got:\n%s", rec.soql)
	}
	if !strings.Contains(rec.soql, "(IsAllDayEvent = false OR Location LIKE '%Head office%')") {
		t.Errorf("the operator's own filters should be the ones ORed, got:\n%s", rec.soql)
	}
}

func TestMatchAllFilterKeepsOneFlatClause(t *testing.T) {
	rec := &queryRecorder{}
	defer rec.serve(t)()

	inputs := append(auth(),
		str("starts_after", "2026-07-01"),
		str("starts_before", "2026-07-07"),
		str("filter_field", "Location"),
		str("filter_operator", "="),
		str("filter_value", "Head office"),
	)
	if _, err := Execute(&core.Flow{}, &core.Node{}, inputs); err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	// One flat ANDed clause, no brackets. (Scope terms lead now that the two
	// lists are built separately; SOQL does not care about term order, and every
	// term is still present and still ANDed.)
	want := "WHERE StartDateTime >= 2026-07-01T00:00:00Z AND StartDateTime <= 2026-07-07T23:59:59Z AND Location = 'Head office'"
	if !strings.Contains(rec.soql, want) {
		t.Errorf("ALL mode should be one flat ANDed clause:\n got %s\nwant ...%s...", rec.soql, want)
	}
	if strings.Contains(rec.soql, "AND (") {
		t.Errorf("ALL mode should not introduce brackets, got:\n%s", rec.soql)
	}
}

// Get Many Events had the same free-text comparison box as Get Many Tasks.
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
	for value := range byValue {
		if _, err := salesforce.ValidateSOQLOperator(value); err != nil {
			t.Errorf("dropdown offers %q, which the builder rejects: %v", value, err)
		}
	}
	if !strings.Contains(byValue["LIKE"], "%") {
		t.Errorf("the LIKE option must tell the operator about the %% wildcard, got %q", byValue["LIKE"])
	}
}

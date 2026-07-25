package crm_salesforce_user_get_all

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
		case strings.HasSuffix(r.URL.Path, "/sobjects/User/describe"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "User",
				"fields": []map[string]interface{}{
					{"name": "IsActive", "type": "boolean"},
					{"name": "Department", "type": "string"},
					{"name": "Title", "type": "string"},
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

// "Active Users Only" is an unconditional promise. Turning on "Match ANY
// filter" used to OR it away, so the rota flow got every active user in the org
// PLUS every deactivated person matching a filter — work assigned to people who
// left. Live: 1 row with the scope ANDed, 16 with it ORed.
func TestMatchAnyFilterDoesNotOrActiveUsersOnly(t *testing.T) {
	rec := &queryRecorder{}
	defer rec.serve(t)()

	inputs := append(auth(),
		&core.Connection{Name: "active_only", Type: core.ConnectionTypeBoolean, Value: true},
		str("filter_field", "Department"),
		str("filter_operator", "="),
		str("filter_value", "Sales"),
		&core.Connection{Name: "filter_conditions", Type: core.ConnectionTypeObject, Value: `[{"field":"Department","operator":"=","value":"Support"}]`},
		&core.Connection{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Value: true},
	)

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}

	if !strings.Contains(rec.soql, "WHERE IsActive = true AND (") {
		t.Errorf("Active Users Only must stay ANDed, got:\n%s", rec.soql)
	}
	if strings.Contains(rec.soql, "IsActive = true OR") {
		t.Errorf("Active Users Only is still being ORed away, got:\n%s", rec.soql)
	}
	if !strings.Contains(rec.soql, "(Department = 'Sales' OR Department = 'Support')") {
		t.Errorf("the operator's own filters should be the ones ORed, got:\n%s", rec.soql)
	}
}

func TestMatchAllFilterKeepsOneFlatClause(t *testing.T) {
	rec := &queryRecorder{}
	defer rec.serve(t)()

	inputs := append(auth(),
		&core.Connection{Name: "active_only", Type: core.ConnectionTypeBoolean, Value: true},
		str("filter_field", "Department"),
		str("filter_operator", "="),
		str("filter_value", "Sales"),
	)
	if _, err := Execute(&core.Flow{}, &core.Node{}, inputs); err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !strings.Contains(rec.soql, "WHERE IsActive = true AND Department = 'Sales'") {
		t.Errorf("ALL mode should be one flat ANDed clause, got:\n%s", rec.soql)
	}
	if strings.Contains(rec.soql, "AND (") {
		t.Errorf("ALL mode should not introduce brackets, got:\n%s", rec.soql)
	}
}

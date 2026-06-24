package databricks_run_sql

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

// strInput builds a connection whose String() returns the given value.
func strInput(name, typ, value string) *core.Connection {
	return &core.Connection{Name: name, Type: typ, Value: value}
}

// TestExecute_SmokeTest stands up a mock Databricks SQL Statement Execution API
// and drives the real Execute() through the full path: submit -> poll a PENDING
// statement to SUCCEEDED -> follow a second result chunk -> shape rows, including
// a NULL cell.
func TestExecute_SmokeTest(t *testing.T) {
	const stmtID = "stmt-123"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/2.0/sql/statements/":
			// Initial submission: still running, forces the poll path.
			_, _ = w.Write([]byte(`{"statement_id":"` + stmtID + `","status":{"state":"PENDING"}}`))

		case r.Method == http.MethodGet && r.URL.Path == "/api/2.0/sql/statements/"+stmtID:
			// Poll: completed, two columns, first chunk of two rows (one NULL),
			// with a link to a second chunk.
			_, _ = w.Write([]byte(`{
				"statement_id":"` + stmtID + `",
				"status":{"state":"SUCCEEDED"},
				"manifest":{"schema":{"columns":[
					{"name":"id","type_text":"INT","position":0},
					{"name":"name","type_text":"STRING","position":1}
				]},"total_row_count":3},
				"result":{"row_count":2,"data_array":[["1","alice"],["2",null]],
					"next_chunk_internal_link":"/api/2.0/sql/statements/` + stmtID + `/result/chunks/1"}
			}`))

		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/result/chunks/1"):
			_, _ = w.Write([]byte(`{"row_count":1,"data_array":[["3","carol"]]}`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	inputs := []*core.Connection{
		strInput("host", core.ConnectionTypeString, srv.URL),
		strInput("token", core.ConnectionTypeSecret, "dapiTEST"),
		strInput("warehouse_id", core.ConnectionTypeString, "wh-1"),
		strInput("statement", core.ConnectionTypeText, "SELECT id, name FROM people"),
	}

	out, err := Execute(nil, nil, inputs)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if got := out["row_count"]; got != 3 {
		t.Errorf("row_count = %v, want 3", got)
	}

	rows, ok := out["results"].([]map[string]interface{})
	if !ok {
		t.Fatalf("results is %T, want []map[string]interface{}", out["results"])
	}
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	if rows[0]["name"] != "alice" {
		t.Errorf("rows[0][name] = %v, want alice", rows[0]["name"])
	}
	if rows[1]["name"] != nil {
		t.Errorf("rows[1][name] = %v, want nil (NULL)", rows[1]["name"])
	}
	if rows[2]["id"] != "3" {
		t.Errorf("rows[2][id] = %v, want 3", rows[2]["id"])
	}

	cols, ok := out["columns"].([]map[string]interface{})
	if !ok || len(cols) != 2 {
		t.Fatalf("columns = %v, want 2 entries", out["columns"])
	}
	if cols[1]["name"] != "name" || cols[1]["type"] != "STRING" {
		t.Errorf("cols[1] = %v, want {name STRING}", cols[1])
	}

	t.Logf("smoke test OK: %v", out["tool_result"])
}

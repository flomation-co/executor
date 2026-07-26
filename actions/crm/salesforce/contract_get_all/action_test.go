package crm_salesforce_contract_get_all

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// recorder answers the describe the typed builder needs and records the SOQL that
// was actually sent — so a test can prove a bad ID never reached the org.
type recorder struct {
	soql string
}

func (rec *recorder) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/sobjects/Contract/describe"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "Contract",
				"fields": []map[string]interface{}{
					{"name": "AccountId", "type": "reference"},
					{"name": "OwnerId", "type": "reference"},
					{"name": "Status", "type": "picklist"},
					{"name": "EndDate", "type": "date"},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/query"):
			rec.soql = r.URL.Query().Get("q")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"totalSize": 0, "done": true, "records": []interface{}{}})
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

func auth(extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
	}, extra...)
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// TestALookupBoxGivenANameIsNamedRatherThanDumpingSOQL is the regression test: a
// company name in Customer (Account), or an email address in Owner, used to be
// dropped straight into the WHERE clause, and Salesforce answered with a SOQL
// fragment, a caret, a row/column position and INVALID_QUERY_FILTER_OPERATOR — a
// code that reads as though the operator's own Filter Comparison were at fault.
func TestALookupBoxGivenANameIsNamedRatherThanDumpingSOQL(t *testing.T) {
	for _, tc := range []struct {
		input string
		value string
		box   string
	}{
		{"account_id", "Edge Communications", "Customer (Account)"},
		{"owner_id", "priya@example.com", "Owner"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			rec := &recorder{}
			defer rec.serve(t)()

			_, err := Execute(&core.Flow{}, &core.Node{}, auth(str(tc.input, tc.value)))
			if err == nil {
				t.Fatalf("%s must be refused before it reaches a WHERE clause", tc.input)
			}
			if !strings.Contains(err.Error(), tc.box) {
				t.Errorf("the message has to name the box the operator filled in, got: %v", err)
			}
			if rec.soql != "" {
				t.Errorf("the query was sent anyway, so Salesforce still answers with a parser dump: %q", rec.soql)
			}
		})
	}
}

// A real ID is untouched: the filter is still applied and the list still runs.
func TestARealIDStillFilters(t *testing.T) {
	rec := &recorder{}
	defer rec.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, auth(str("account_id", "001aj00003CnM29AAF")))
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}
	if !strings.Contains(rec.soql, "AccountId = '001aj00003CnM29AAF'") {
		t.Errorf("the account filter should still be in the query, got: %q", rec.soql)
	}
}

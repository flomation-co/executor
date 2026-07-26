package crm_salesforce_quote_get_all

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
// was sent, so the default projection can be inspected.
type recorder struct {
	soql string
}

func (rec *recorder) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/sobjects/Quote/describe"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "Quote",
				"fields": []map[string]interface{}{
					{"name": "Status", "type": "picklist"},
					{"name": "ExpirationDate", "type": "date"},
					{"name": "GrandTotal", "type": "currency"},
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

// TestTheDefaultProjectionCarriesTheCustomerEitherWay is the regression test.
// Quote.AccountId is read-only and derived from the parent opportunity, so it is
// null on every standalone quote — and a standalone quote is exactly what Create
// Quote's own "Company (Quote Account)" input produces, because it writes
// QuoteAccountId instead. With only AccountId in the projection, "list quotes
// expiring this month, email the customer" came back with an empty customer on
// every row and no error anywhere.
func TestTheDefaultProjectionCarriesTheCustomerEitherWay(t *testing.T) {
	rec := &recorder{}
	defer rec.serve(t)()

	inputs := []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
	}
	if _, err := Execute(&core.Flow{}, &core.Node{}, inputs); err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	for _, want := range []string{"QuoteAccountId", "QuoteAccount.Name", "AccountId", "Account.Name"} {
		if !strings.Contains(rec.soql, want) {
			t.Errorf("the default projection must carry %s so the list names the customer: %q", want, rec.soql)
		}
	}
}

// TestTheContainsOptionTellsTheOperatorAboutTheWildcard: SOQL LIKE without a %
// is an exact match, so "Contains (LIKE)" quietly returned nothing for a value
// that is genuinely a substring — reported as a confident "Found 0 quotes". The
// same assertion already guards task_get_all and event_get_all.
func TestTheContainsOptionTellsTheOperatorAboutTheWildcard(t *testing.T) {
	var operator *core.Connection
	for i := range Inputs {
		if Inputs[i].Name == "filter_operator" {
			operator = &Inputs[i]
		}
	}
	if operator == nil {
		t.Fatal("filter_operator input is missing")
	}
	byValue := map[string]string{}
	for _, o := range operator.Options {
		byValue[o.Value] = o.Name
	}
	for _, op := range []string{"LIKE", "NOT LIKE"} {
		if !strings.Contains(byValue[op], "%") {
			t.Errorf("the %s option must tell the operator about the %% wildcard, got %q", op, byValue[op])
		}
	}
}

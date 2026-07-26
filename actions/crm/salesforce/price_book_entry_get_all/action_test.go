package crm_salesforce_price_book_entry_get_all

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
// was sent, so a test can prove a cross-typed ID never reached the org.
type recorder struct {
	soql string
}

func (rec *recorder) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/sobjects/PricebookEntry/describe"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "PricebookEntry",
				"fields": []map[string]interface{}{
					{"name": "Pricebook2Id", "type": "reference"},
					{"name": "Product2Id", "type": "reference"},
					{"name": "UnitPrice", "type": "currency"},
					{"name": "IsActive", "type": "boolean"},
					{"name": "ProductCode", "type": "string"},
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

// TestACrossTypedIDIsRefusedRatherThanReturningZeroRows is the regression test.
//
// A cross-typed ID is the one mistake SOQL stays SILENT about: a name in an ID
// filter is a loud 400, but a well-formed ID of the WRONG object returns zero rows
// with no error (verified live — Pricebook2Id = a Product ID gives totalSize 0).
// Shape validation cannot catch it, because every Salesforce ID is 15 or 18
// alphanumerics, so the action reported "Found no prices for price book 01taj…, add
// it with Add Product to Price Book" and sent the operator off to fix a price book
// that was never a price book.
func TestACrossTypedIDIsRefusedRatherThanReturningZeroRows(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		value    string
		wantBox  string
		wantHint string
	}{
		{"product ID in the Price Book box", "pricebook_id", "01taj00000UI9YEAA1", "Price Book", "belongs in the Product box"},
		{"price book ID in the Product box", "product_id", "01saj00000LXMP3AAP", "Product", "belongs in the Price Book box"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			defer rec.serve(t)()

			_, err := Execute(&core.Flow{}, &core.Node{}, auth(str(tc.input, tc.value)))
			if err == nil {
				t.Fatal("a cross-typed ID must be refused rather than silently matching nothing")
			}
			if !strings.Contains(err.Error(), tc.wantBox) {
				t.Errorf("the message has to name the box that was filled in, got: %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantHint) {
				t.Errorf("the message has to say where the ID does belong, got: %v", err)
			}
			if rec.soql != "" {
				t.Errorf("a query was sent anyway, so the zero-row misdiagnosis still happens: %q", rec.soql)
			}
		})
	}
}

// The right IDs in the right boxes are untouched.
func TestTheRightIDsStillScopeTheList(t *testing.T) {
	rec := &recorder{}
	defer rec.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, auth(
		str("pricebook_id", "01saj00000LXMP3AAP"),
		str("product_id", "01taj00000UI9YEAA1"),
	))
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}
	if !strings.Contains(rec.soql, "Pricebook2Id = '01saj00000LXMP3AAP'") || !strings.Contains(rec.soql, "Product2Id = '01taj00000UI9YEAA1'") {
		t.Errorf("both scope filters should still be in the query, got: %q", rec.soql)
	}
}

// TestContainsNamesTheWildcard: SOQL LIKE without a % is an exact match, so
// "Contains (LIKE)" quietly returned nothing for a genuine substring.
func TestContainsNamesTheWildcard(t *testing.T) {
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

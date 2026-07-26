package crm_salesforce_object_describe

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// describeAccount is the shape a real Account describe comes back in. Note
// picklistValues: Salesforce sends the key on EVERY field, as an empty array
// for the ones that are not dropdowns — which is what made the old
// `values == nil` guard unreachable for them.
func describeAccount(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/sobjects/Account/describe") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "Account",
			"fields": []map[string]interface{}{
				{"name": "Name", "label": "Account Name", "type": "string", "picklistValues": []interface{}{}},
				{"name": "AnnualRevenue", "label": "Annual Revenue", "type": "currency", "picklistValues": []interface{}{}},
				{"name": "OwnerId", "label": "Owner ID", "type": "reference", "picklistValues": []interface{}{}},
				{"name": "Industry", "label": "Industry", "type": "picklist", "picklistValues": []interface{}{
					map[string]interface{}{"label": "Manufacturing", "value": "Manufacturing", "active": true},
					map[string]interface{}{"label": "Retail", "value": "Retail", "active": true},
				}},
				{"name": "Empty_Choice__c", "label": "Empty Choice", "type": "picklist", "picklistValues": []interface{}{}},
			},
			"childRelationships": []interface{}{},
		})
	}))
	restore := salesforce.SetHostForTest(srv.URL)
	return func() {
		restore()
		srv.Close()
	}
}

func inputs(field string) []*core.Connection {
	return []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "object", Type: core.ConnectionTypeString, Value: "Account"},
		{Name: "picklist_field", Type: core.ConnectionTypeString, Value: field},
	}
}

// The regression: asking about a field that exists but is not a dropdown used
// to report success with `Described Account: Name accepts 0 value(s)` and
// picklistValues: [] — affirmatively false metadata (AnnualRevenue accepts any
// number), handed to an operator or, on the AI-tools path, to a model whose
// entire observation is that one line.
func TestNonPicklistFieldIsRejectedWithItsRealType(t *testing.T) {
	defer describeAccount(t)()

	for _, c := range []struct{ field, wantType string }{
		{"Name", "text field"},
		{"AnnualRevenue", "money field"},
		{"OwnerId", "link to another record"},
	} {
		out, err := Execute(&core.Flow{}, &core.Node{}, inputs(c.field))
		if err == nil {
			t.Fatalf("%s: expected a hard error, got success: %v", c.field, out["tool_result"])
		}
		if out != nil {
			t.Errorf("%s: a configuration mistake must return a nil result", c.field)
		}
		if !strings.Contains(err.Error(), "is not a dropdown") {
			t.Errorf("%s: error should say it is not a dropdown, got %q", c.field, err)
		}
		if !strings.Contains(err.Error(), c.wantType) {
			t.Errorf("%s: error should name the real type %q, got %q", c.field, c.wantType, err)
		}
	}
}

// The absent-field message must stay distinct — and must no longer claim the
// field might merely "not be a dropdown", which was the guess it made when it
// was covering both cases.
func TestAbsentFieldSaysSo(t *testing.T) {
	defer describeAccount(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs("Nope__c"))
	if err == nil {
		t.Fatalf("expected a hard error, got %v", out)
	}
	if !strings.Contains(err.Error(), `has no field called "Nope__c"`) {
		t.Errorf("unexpected message: %q", err)
	}
	if strings.Contains(err.Error(), "is not a dropdown") {
		t.Errorf("the missing-field message should not also guess at the type: %q", err)
	}
}

func TestRealPicklistStillWorks(t *testing.T) {
	defer describeAccount(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs("Industry"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, _ := out["tool_result"].(string); got != "Described Account: Industry accepts 2 value(s)" {
		t.Errorf("unexpected summary: %q", got)
	}
	result, _ := out["result"].(map[string]interface{})
	values, _ := result["picklistValues"].([]interface{})
	if len(values) != 2 {
		t.Errorf("expected the 2 picklist values to be lifted out, got %d", len(values))
	}
}

// A genuine dropdown with nothing in it is not an error — it is an org that has
// not set the values up, and the summary should say that rather than the
// "accepts 0 value(s)" that reads like a bug.
func TestEmptyPicklistIsReportedHonestly(t *testing.T) {
	defer describeAccount(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs("Empty_Choice__c"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := out["tool_result"].(string)
	if !strings.Contains(got, "no values set up in your org") {
		t.Errorf("unexpected summary: %q", got)
	}
}

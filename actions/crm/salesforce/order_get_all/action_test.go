package crm_salesforce_order_get_all

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

// TestContainsNamesTheWildcard is the regression test. SOQL LIKE without a % is an
// exact (case-insensitive) match, not a substring match — verified live,
// `Name LIKE 'GenWatt'` returns nothing while `Name LIKE '%GenWatt%'` returns nine
// products. Labelled only "Contains (LIKE)", the option quietly returned nothing
// for a value that genuinely is a substring, and reported a confident "Found 0".
// The same assertion already guards task_get_all and event_get_all.
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

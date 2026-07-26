package crm_salesforce_custom_object_get_all

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

// TestContainsNamesTheWildcard: SOQL LIKE without a % is an exact
// (case-insensitive) match, not a substring match — verified live, `Name LIKE
// 'GenWatt'` returns nothing where `Name LIKE '%GenWatt%'` returns nine rows.
// Labelled only "Contains (LIKE)", the option silently under-returned and the
// action reported a confident "Found 0". Seven Get Many actions in this node
// already carry the wildcard wording; this keeps all of them saying it.
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

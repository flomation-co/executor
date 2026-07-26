package crm_salesforce_price_book_get_all

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

func inputByName(t *testing.T, name string) *core.Connection {
	t.Helper()
	for i := range Inputs {
		if Inputs[i].Name == name {
			return &Inputs[i]
		}
	}
	t.Fatalf("%s input is missing", name)
	return nil
}

// TestContainsNamesTheWildcardAndTheExampleWorks is the regression test.
//
// SOQL LIKE without a % is an exact (case-insensitive) match, not a substring
// match: verified live, `Name LIKE 'Trade'` returns nothing while
// `Name LIKE '%Trade%'` returns the book "Trade Prices". Nothing told the operator
// that — and this action's Filter Value placeholder actively taught the broken
// form ("Trade with Contains"), so an office manager copying it read a confident
// "Found 0 price books" and concluded the book did not exist.
func TestContainsNamesTheWildcardAndTheExampleWorks(t *testing.T) {
	operator := inputByName(t, "filter_operator")
	byValue := map[string]string{}
	for _, o := range operator.Options {
		byValue[o.Value] = o.Name
	}
	for _, op := range []string{"LIKE", "NOT LIKE"} {
		if !strings.Contains(byValue[op], "%") {
			t.Errorf("the %s option must tell the operator about the %% wildcard, got %q", op, byValue[op])
		}
	}

	value := inputByName(t, "filter_value")
	if strings.Contains(value.Placeholder, "Contains") && !strings.Contains(value.Placeholder, "%") {
		t.Errorf("the Filter Value example must use the wildcard it needs, got %q", value.Placeholder)
	}
}

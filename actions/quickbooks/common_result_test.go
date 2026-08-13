package quickbooks_common

import (
	"strings"
	"testing"
)

func TestObjectResultEmbedsData(t *testing.T) {
	obj := map[string]interface{}{"Id": "42", "DisplayName": "Acme Widgets"}
	res := ObjectResult("42", obj, "Created customer")

	tr, ok := res["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result not a string: %T", res["tool_result"])
	}
	if !strings.Contains(tr, "Created customer") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "Acme Widgets") {
		t.Fatalf("tool_result missing data value: %q", tr)
	}
	if res["result"] == nil {
		t.Fatalf("result output not preserved")
	}
	if r, ok := res["result"].(map[string]interface{}); !ok || r["DisplayName"] != "Acme Widgets" {
		t.Fatalf("result output not preserved: %#v", res["result"])
	}
}

func TestListResultEmbedsData(t *testing.T) {
	items := []map[string]interface{}{
		{"Id": "1", "Name": "Zephyr Ltd"},
		{"Id": "2", "Name": "Other"},
	}
	res := ListResult(items, "Found 2 customers")

	tr, ok := res["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result not a string: %T", res["tool_result"])
	}
	if !strings.Contains(tr, "Found 2 customers") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "Zephyr Ltd") {
		t.Fatalf("tool_result missing data value: %q", tr)
	}
	if res["count"] != 2 {
		t.Fatalf("count not preserved: %#v", res["count"])
	}
	if res["results"] == nil {
		t.Fatalf("results output not preserved")
	}
}

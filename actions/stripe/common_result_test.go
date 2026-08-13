package stripe_common

import (
	"strings"
	"testing"
)

// TestObjectResultToolResultCarriesData verifies that ObjectResult embeds the
// object's data into tool_result (so an AI caller reading the verbatim
// tool_result gets both the summary and the underlying data) while leaving the
// structured "result" output intact.
func TestObjectResultToolResultCarriesData(t *testing.T) {
	obj := map[string]interface{}{
		"id":    "cus_ABC123",
		"email": "distinctive@example.com",
	}
	out := ObjectResult(obj, "Customer created")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Customer created") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "distinctive@example.com") {
		t.Fatalf("tool_result missing embedded data value: %q", tr)
	}

	res, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result output not preserved as map: %T", out["result"])
	}
	if res["email"] != "distinctive@example.com" {
		t.Fatalf("structured result output not preserved: %v", res["email"])
	}
	if out["id"] != "cus_ABC123" {
		t.Fatalf("id output not preserved: %v", out["id"])
	}
}

// TestListResultToolResultCarriesData verifies that ListResult embeds the item
// data into tool_result while leaving results/count/has_more intact.
func TestListResultToolResultCarriesData(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "in_DISTINCTIVE1", "amount": 4242},
	}
	out := ListResult(items, true, "Found invoices")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Found invoices") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "in_DISTINCTIVE1") {
		t.Fatalf("tool_result missing embedded data value: %q", tr)
	}

	if out["count"] != 1 {
		t.Fatalf("count output not preserved: %v", out["count"])
	}
	if out["has_more"] != true {
		t.Fatalf("has_more output not preserved: %v", out["has_more"])
	}
	if _, ok := out["results"].([]map[string]interface{}); !ok {
		t.Fatalf("results output not preserved: %T", out["results"])
	}
}

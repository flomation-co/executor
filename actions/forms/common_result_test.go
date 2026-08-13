package forms_common

import (
	"strings"
	"testing"
)

// TestObjectResultEmbedsData verifies that ObjectResult's tool_result contains
// both the human summary and the underlying object data, so an AI caller
// receives the payload (the engine never falls through a non-empty tool_result).
func TestObjectResultEmbedsData(t *testing.T) {
	obj := map[string]interface{}{
		"id":   "resp_123",
		"name": "Zaphod Beeblebrox",
	}
	summary := "Fetched response resp_123"

	out := ObjectResult(obj, summary)

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, summary) {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "Zaphod Beeblebrox") {
		t.Fatalf("tool_result missing distinctive data value: %q", tr)
	}

	res, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result is not a map: %T", out["result"])
	}
	if res["name"] != "Zaphod Beeblebrox" {
		t.Fatalf("structured result not preserved: %+v", res)
	}
	if out["id"] != "resp_123" {
		t.Fatalf("id not preserved: %v", out["id"])
	}
}

// TestListResultEmbedsData verifies that ListResult's tool_result contains both
// the summary and the item data, and that the structured list is preserved.
func TestListResultEmbedsData(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "a1", "title": "Trillian Astra"},
		{"id": "a2", "title": "Ford Prefect"},
	}
	summary := "Listed 2 forms"

	out := ListResult(items, summary)

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, summary) {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "Trillian Astra") {
		t.Fatalf("tool_result missing distinctive data value: %q", tr)
	}

	results, ok := out["results"].([]map[string]interface{})
	if !ok {
		t.Fatalf("results is not a slice of maps: %T", out["results"])
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if out["count"] != 2 {
		t.Fatalf("count not preserved: %v", out["count"])
	}
}

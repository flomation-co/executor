package hubspot_common

import (
	"strings"
	"testing"
)

// TestObjectResultToolResultEmbedsData verifies ObjectResult's tool_result
// carries BOTH the human summary and a JSON encoding of the record data, and
// that the structured output keys are preserved.
func TestObjectResultToolResultEmbedsData(t *testing.T) {
	obj := map[string]interface{}{
		"id": "12345",
		"properties": map[string]interface{}{
			"email": "distinctive@example.com",
		},
	}
	out := ObjectResult(obj, "Contact created")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %#v", out["tool_result"])
	}
	if !strings.Contains(tr, "Contact created") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "distinctive@example.com") {
		t.Fatalf("tool_result missing embedded data value: %q", tr)
	}

	if out["id"] != "12345" {
		t.Fatalf("structured id output not preserved: %#v", out["id"])
	}
	if out["success"] != true {
		t.Fatalf("success output not preserved: %#v", out["success"])
	}
	if out["result"] == nil {
		t.Fatalf("result output not preserved")
	}
}

// TestListResultToolResultEmbedsData verifies ListResult's tool_result carries
// both the summary and the results-slice data, and preserves count/after.
func TestListResultToolResultEmbedsData(t *testing.T) {
	resp := map[string]interface{}{
		"results": []interface{}{
			map[string]interface{}{"id": "1", "name": "distinctive-row"},
		},
		"paging": map[string]interface{}{
			"next": map[string]interface{}{"after": "cursor-99"},
		},
	}
	out := ListResult(resp, "Found 1 contact")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %#v", out["tool_result"])
	}
	if !strings.Contains(tr, "Found 1 contact") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "distinctive-row") {
		t.Fatalf("tool_result missing embedded list data value: %q", tr)
	}

	if out["count"] != 1 {
		t.Fatalf("count output not preserved: %#v", out["count"])
	}
	if out["after"] != "cursor-99" {
		t.Fatalf("after output not preserved: %#v", out["after"])
	}
	if out["success"] != true {
		t.Fatalf("success output not preserved: %#v", out["success"])
	}
}

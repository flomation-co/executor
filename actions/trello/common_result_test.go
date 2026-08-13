package trello_common

import (
	"strings"
	"testing"
)

// TestResourceResultEmbedsData verifies that a single-resource success return
// (ResourceResult) surfaces BOTH the human summary and the structured object in
// tool_result, and preserves the structured output under "result".
func TestResourceResultEmbedsData(t *testing.T) {
	obj := map[string]interface{}{
		"id":   "card123",
		"name": "Distinctive-Card-Name",
	}
	out := ResourceResult(obj, "Created card")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Created card") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "Distinctive-Card-Name") {
		t.Fatalf("tool_result missing embedded data value: %q", tr)
	}
	if _, ok := out["result"].(map[string]interface{}); !ok {
		t.Fatalf("structured result output not preserved: %T", out["result"])
	}
	if out["success"] != true {
		t.Fatalf("expected success true, got %v", out["success"])
	}
}

// TestSuccessResultEmbedsData verifies that SuccessResult (id-known operations)
// embeds the result map's distinctive value into tool_result.
func TestSuccessResultEmbedsData(t *testing.T) {
	result := map[string]interface{}{
		"archived": true,
		"marker":   "Distinctive-Success-Marker",
	}
	out := SuccessResult("card123", result, "Archived card")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Archived card") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "Distinctive-Success-Marker") {
		t.Fatalf("tool_result missing embedded data value: %q", tr)
	}
	if _, ok := out["result"].(map[string]interface{}); !ok {
		t.Fatalf("structured result output not preserved: %T", out["result"])
	}
}

// TestListResultEmbedsData verifies that a collection success return (ListResult)
// surfaces both the summary and a distinctive item value in tool_result, and
// preserves the structured items under "results".
func TestListResultEmbedsData(t *testing.T) {
	items := []interface{}{
		map[string]interface{}{"id": "1", "name": "Distinctive-Item-Name"},
		map[string]interface{}{"id": "2", "name": "Other"},
	}
	out := ListResult(items, "Found 2 boards")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Found 2 boards") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "Distinctive-Item-Name") {
		t.Fatalf("tool_result missing embedded data value: %q", tr)
	}
	if _, ok := out["results"].([]interface{}); !ok {
		t.Fatalf("structured results output not preserved: %T", out["results"])
	}
	if out["count"] != 2 {
		t.Fatalf("expected count 2, got %v", out["count"])
	}
}

package jira_common

import (
	"strings"
	"testing"
)

// TestResourceResultToolResultEmbedsData asserts the single-object success helper
// folds the underlying object into tool_result (summary + distinctive data value)
// while leaving the structured "result" output intact.
func TestResourceResultToolResultEmbedsData(t *testing.T) {
	obj := map[string]interface{}{"key": "SCRUM-42", "summary": "distinctive-value-alpha"}
	out := ResourceResult(obj, "Created issue SCRUM-42")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Created issue SCRUM-42") {
		t.Fatalf("tool_result missing summary, got: %q", tr)
	}
	if !strings.Contains(tr, "distinctive-value-alpha") {
		t.Fatalf("tool_result missing embedded data value, got: %q", tr)
	}
	if _, ok := out["result"].(map[string]interface{}); !ok {
		t.Fatalf("structured result output not preserved: %T", out["result"])
	}
	if out["id"] != "SCRUM-42" {
		t.Fatalf("id output not preserved, got: %v", out["id"])
	}
}

// TestSuccessResultToolResultEmbedsData covers the no-body/known-id success helper.
func TestSuccessResultToolResultEmbedsData(t *testing.T) {
	result := map[string]interface{}{"status": "distinctive-value-beta"}
	out := SuccessResult("SCRUM-7", result, "Updated issue SCRUM-7")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Updated issue SCRUM-7") {
		t.Fatalf("tool_result missing summary, got: %q", tr)
	}
	if !strings.Contains(tr, "distinctive-value-beta") {
		t.Fatalf("tool_result missing embedded data value, got: %q", tr)
	}
	if _, ok := out["result"].(map[string]interface{}); !ok {
		t.Fatalf("structured result output not preserved: %T", out["result"])
	}
}

// TestListResultToolResultEmbedsItems asserts the list helper embeds the items
// array into tool_result while preserving results/count/total outputs.
func TestListResultToolResultEmbedsItems(t *testing.T) {
	items := []interface{}{
		map[string]interface{}{"key": "distinctive-value-gamma"},
	}
	out := ListResult(items, 5, "Found 1 of 5 issues")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Found 1 of 5 issues") {
		t.Fatalf("tool_result missing summary, got: %q", tr)
	}
	if !strings.Contains(tr, "distinctive-value-gamma") {
		t.Fatalf("tool_result missing embedded item value, got: %q", tr)
	}
	if res, ok := out["results"].([]interface{}); !ok || len(res) != 1 {
		t.Fatalf("results output not preserved: %#v", out["results"])
	}
	if out["count"] != 1 {
		t.Fatalf("count output changed, got: %v", out["count"])
	}
	if out["total"] != 5 {
		t.Fatalf("total output changed, got: %v", out["total"])
	}
}

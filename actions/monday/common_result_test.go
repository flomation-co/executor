package monday_common

import (
	"strings"
	"testing"
)

// TestResourceResultToolResultEmbedsData verifies the single-object success
// helper folds the object's data into tool_result (so AI callers get the data,
// not just the summary) while preserving the structured "result" output.
func TestResourceResultToolResultEmbedsData(t *testing.T) {
	obj := map[string]interface{}{"id": "123456789", "name": "Marketing Board"}
	out := ResourceResult(obj, "Created board 123456789")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %#v", out["tool_result"])
	}
	if !strings.Contains(tr, "Created board 123456789") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "Marketing Board") {
		t.Fatalf("tool_result missing embedded data value: %q", tr)
	}

	res, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result output not preserved: %#v", out["result"])
	}
	if res["name"] != "Marketing Board" {
		t.Fatalf("structured result mangled: %#v", res)
	}
}

// TestListResultToolResultEmbedsData verifies the collection success helper
// folds the items into tool_result while preserving the structured "results".
func TestListResultToolResultEmbedsData(t *testing.T) {
	items := []interface{}{
		map[string]interface{}{"id": "1", "name": "Task Alpha"},
		map[string]interface{}{"id": "2", "name": "Task Beta"},
	}
	out := ListResult(items, "Found 2 items")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %#v", out["tool_result"])
	}
	if !strings.Contains(tr, "Found 2 items") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "Task Alpha") {
		t.Fatalf("tool_result missing embedded data value: %q", tr)
	}

	results, ok := out["results"].([]interface{})
	if !ok {
		t.Fatalf("results output not preserved: %#v", out["results"])
	}
	if len(results) != 2 {
		t.Fatalf("structured results mangled: %#v", results)
	}
}

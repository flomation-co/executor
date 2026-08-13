package asana_common

import (
	"strings"
	"testing"
)

func TestResourceResultEmbedsData(t *testing.T) {
	obj := map[string]interface{}{"gid": "12345", "name": "Design homepage"}
	out := ResourceResult(obj, "Task fetched")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Task fetched") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "Design homepage") {
		t.Fatalf("tool_result missing data value: %q", tr)
	}
	if _, ok := out["result"].(map[string]interface{}); !ok {
		t.Fatalf("result output not preserved: %T", out["result"])
	}
}

func TestSuccessResultEmbedsData(t *testing.T) {
	result := map[string]interface{}{"status": "added", "tag": "urgent-42"}
	out := SuccessResult("999", result, "Tag added")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Tag added") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "urgent-42") {
		t.Fatalf("tool_result missing data value: %q", tr)
	}
	if _, ok := out["result"].(map[string]interface{}); !ok {
		t.Fatalf("result output not preserved: %T", out["result"])
	}
}

func TestListResultEmbedsData(t *testing.T) {
	items := []interface{}{
		map[string]interface{}{"gid": "1", "name": "distinctive-item-xyz"},
	}
	out := ListResult(items, "Found 1 task")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Found 1 task") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "distinctive-item-xyz") {
		t.Fatalf("tool_result missing data value: %q", tr)
	}
	if _, ok := out["results"].([]interface{}); !ok {
		t.Fatalf("results output not preserved: %T", out["results"])
	}
}

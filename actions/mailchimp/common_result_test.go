package mailchimp_common

import (
	"strings"
	"testing"
)

// TestObjectResultToolResultEmbedsData asserts ObjectResult's tool_result carries
// both the human summary AND the structured payload, and preserves the raw
// object under "result".
func TestObjectResultToolResultEmbedsData(t *testing.T) {
	obj := map[string]interface{}{
		"id":          "campaign_123",
		"subject_line": "Distinctive-Subject-Zebra",
	}
	out := ObjectResult(obj, "Fetched campaign")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Fetched campaign") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "Distinctive-Subject-Zebra") {
		t.Fatalf("tool_result missing embedded data value: %q", tr)
	}

	res, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result output not preserved as map: %T", out["result"])
	}
	if res["subject_line"] != "Distinctive-Subject-Zebra" {
		t.Fatalf("structured result lost data: %+v", res)
	}
}

// TestListResultToolResultEmbedsItems asserts ListResult's tool_result carries
// both the summary AND the parsed items slice, and preserves structured outputs.
func TestListResultToolResultEmbedsItems(t *testing.T) {
	items := []interface{}{
		map[string]interface{}{"id": "m1", "email_address": "distinctive-member-quokka@example.com"},
	}
	raw := map[string]interface{}{"total_items": 1}
	out := ListResult(items, 1, raw, "Listed members")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Listed members") {
		t.Fatalf("tool_result missing summary: %q", tr)
	}
	if !strings.Contains(tr, "distinctive-member-quokka@example.com") {
		t.Fatalf("tool_result missing embedded item value: %q", tr)
	}

	results, ok := out["results"].([]interface{})
	if !ok {
		t.Fatalf("results output not preserved as slice: %T", out["results"])
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if out["count"] != 1 {
		t.Fatalf("expected count 1, got %v", out["count"])
	}
}

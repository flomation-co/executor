package airtable_common

import (
	"strings"
	"testing"
)

// TestRecordResultEmbedsData verifies RecordResult puts the record payload
// into tool_result alongside the summary, so an AI caller (which reads
// tool_result verbatim and never falls through to the data outputs) actually
// receives the record data.
func TestRecordResultEmbedsData(t *testing.T) {
	rec := map[string]interface{}{
		"id": "recABC123",
		"fields": map[string]interface{}{
			"Name": "Acme Widgets",
		},
	}
	out := RecordResult(rec, "Fetched record recABC123")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Fetched record recABC123") {
		t.Fatalf("tool_result missing summary text: %q", tr)
	}
	if !strings.Contains(tr, "Acme Widgets") {
		t.Fatalf("tool_result missing record data (field value): %q", tr)
	}

	// Structured data outputs must remain present for flow wiring.
	if out["record"] == nil {
		t.Fatalf("record output key missing")
	}
	if out["id"] != "recABC123" {
		t.Fatalf("id output key wrong: %v", out["id"])
	}
	if out["fields"] == nil {
		t.Fatalf("fields output key missing")
	}
}

// TestListResultEmbedsData verifies ListResult embeds the parsed records slice
// in tool_result while keeping the structured outputs untouched.
func TestListResultEmbedsData(t *testing.T) {
	records := []interface{}{
		map[string]interface{}{
			"id": "rec001",
			"fields": map[string]interface{}{
				"Name": "Zephyr Corp",
			},
		},
	}
	raw := map[string]interface{}{"records": records}
	out := ListResult(records, "off42", raw, "Listed 1 record")

	tr, ok := out["tool_result"].(string)
	if !ok {
		t.Fatalf("tool_result is not a string: %T", out["tool_result"])
	}
	if !strings.Contains(tr, "Listed 1 record") {
		t.Fatalf("tool_result missing summary text: %q", tr)
	}
	if !strings.Contains(tr, "Zephyr Corp") {
		t.Fatalf("tool_result missing record data (field value): %q", tr)
	}

	// Structured data outputs must remain present for flow wiring.
	if out["records"] == nil {
		t.Fatalf("records output key missing")
	}
	if out["count"] != 1 {
		t.Fatalf("count output key wrong: %v", out["count"])
	}
	if out["offset"] != "off42" {
		t.Fatalf("offset output key wrong: %v", out["offset"])
	}
	if out["result"] == nil {
		t.Fatalf("result output key missing")
	}
}

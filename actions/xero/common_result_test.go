package xero_common

import (
	"strings"
	"testing"
)

// TestObjectResultEmbedsData is the regression guard for the bug where Xero
// report/get actions returned only a static summary in tool_result (the
// engine's tool-result fallback uses tool_result verbatim when non-empty and
// never falls through to the `result` output), so an AI caller received
// "Fetched … report" with none of the actual figures.
func TestObjectResultEmbedsData(t *testing.T) {
	obj := map[string]interface{}{"Reports": []interface{}{map[string]interface{}{"ReportName": "ProfitAndLoss"}}}
	out := ObjectResult("", obj, "Fetched Xero Profit and Loss report")

	tr, _ := out["tool_result"].(string)
	if !strings.Contains(tr, "Fetched Xero Profit and Loss report") {
		t.Fatalf("tool_result lost the summary: %q", tr)
	}
	if !strings.Contains(tr, "ProfitAndLoss") {
		t.Fatalf("tool_result must carry the report data so the AI can read it, got: %q", tr)
	}
	// The structured object is still available for downstream flow wiring.
	if _, ok := out["result"].(map[string]interface{}); !ok {
		t.Fatalf("result object should be preserved for wiring")
	}
}

func TestListResultEmbedsData(t *testing.T) {
	items := []map[string]interface{}{{"Name": "Acme Ltd"}, {"Name": "Globex"}}
	out := ListResult(items, "Found 2 contacts")

	tr, _ := out["tool_result"].(string)
	if !strings.Contains(tr, "Acme Ltd") || !strings.Contains(tr, "Globex") {
		t.Fatalf("list tool_result must carry the row data, got: %q", tr)
	}
	if out["count"].(int) != 2 {
		t.Fatalf("count should be preserved, got %v", out["count"])
	}
}

// A nil/empty payload must not corrupt the summary.
func TestSummaryWithDataNilFallback(t *testing.T) {
	if got := summaryWithData("only a summary", nil); got != "only a summary" {
		t.Fatalf("nil data should leave the summary untouched, got %q", got)
	}
}

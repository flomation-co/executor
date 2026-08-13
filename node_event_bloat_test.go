package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestTruncateEventValues_Deep is the regression guard for log bloat: a large
// string nested inside a slice/map (e.g. a base64 image block or tool_result
// content) must be truncated, not passed through whole.
func TestTruncateEventValues_Deep(t *testing.T) {
	big := strings.Repeat("A", maxEventStringBytes+500)
	in := map[string]interface{}{
		"nested": []interface{}{
			map[string]interface{}{"image": big},
		},
		"small": "ok",
	}
	out := truncateEventValues(in)
	b, _ := json.Marshal(out)
	if strings.Contains(string(b), big) {
		t.Errorf("nested oversized string was not truncated")
	}
	if !strings.Contains(string(b), "truncated") {
		t.Errorf("expected a truncation marker; got %s", b)
	}
	if !strings.Contains(string(b), `"ok"`) {
		t.Errorf("small value should survive")
	}
}

// TestCapEventField guards the many-small-values case (the tool-schema set):
// a field whose whole serialised size is too large is summarised.
func TestCapEventField(t *testing.T) {
	// Many small entries — no single string is oversized, but the total is.
	schemas := make([]interface{}, 0, 200)
	for i := 0; i < 200; i++ {
		schemas = append(schemas, map[string]interface{}{"name": "tool", "desc": strings.Repeat("x", 300)})
	}
	got := capEventField(map[string]interface{}{"tool_definitions": schemas})
	s, ok := got.(string)
	if !ok || !strings.Contains(s, "omitted") {
		t.Errorf("oversized field should be summarised, got %T %v", got, got)
	}
	// Small field is unchanged.
	small := map[string]interface{}{"x": "y"}
	if _, ok := capEventField(small).(string); ok {
		t.Errorf("small field should pass through unchanged")
	}
}

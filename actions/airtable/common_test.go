package airtable_common

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

func conn(name, typ string, value interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: typ, Value: value}
}

func TestBuildFields_KVOverlaysJSONAndPreservesTypes(t *testing.T) {
	inputs := []*core.Connection{
		conn("fields", core.ConnectionTypeObject, `{"Name":"Ada","Count":3,"Tags":["x","y"]}`),
		conn("fields_kv", core.ConnectionTypeKeyValueArray, `[{"key":"Name","value":"Grace"},{"key":"City","value":"NYC"}]`),
	}
	got, err := BuildFields(inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["Name"] != "Grace" {
		t.Errorf("key/value should override JSON; Name=%v", got["Name"])
	}
	if got["City"] != "NYC" {
		t.Errorf("City=%v", got["City"])
	}
	if _, ok := got["Tags"].([]interface{}); !ok {
		t.Errorf("Tags should stay a JSON array, got %T", got["Tags"])
	}
	if got["Count"] == nil {
		t.Errorf("Count should be preserved from JSON")
	}
}

func TestBuildFields_ParsedMapValue(t *testing.T) {
	// When the engine has already parsed the object input into a map.
	inputs := []*core.Connection{
		conn("fields", core.ConnectionTypeObject, map[string]interface{}{"Name": "Ada"}),
	}
	got, err := BuildFields(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if got["Name"] != "Ada" {
		t.Errorf("Name=%v", got["Name"])
	}
}

func TestBuildFields_Empty(t *testing.T) {
	got, err := BuildFields(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestBuildFields_InvalidJSON(t *testing.T) {
	inputs := []*core.Connection{conn("fields", core.ConnectionTypeObject, `{not json`)}
	if _, err := BuildFields(inputs); err == nil {
		t.Error("expected an error for invalid JSON object")
	}
}

func TestBuildListQuery_SimpleSortFieldsAndFormula(t *testing.T) {
	inputs := []*core.Connection{
		conn("filter_by_formula", core.ConnectionTypeString, "NOT({Name}='')"),
		conn("return_fields", core.ConnectionTypeString, "Name, Email ,Age"),
		conn("sort_field", core.ConnectionTypeString, "Name"),
		conn("sort_direction", core.ConnectionTypeString, "desc"),
	}
	q, err := BuildListQuery(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("filterByFormula") != "NOT({Name}='')" {
		t.Errorf("filterByFormula=%q", q.Get("filterByFormula"))
	}
	if fields := q["fields[]"]; len(fields) != 3 {
		t.Errorf("expected 3 projected fields (trimmed), got %v", fields)
	}
	if q.Get("sort[0][field]") != "Name" || q.Get("sort[0][direction]") != "desc" {
		t.Errorf("sort=%q/%q", q.Get("sort[0][field]"), q.Get("sort[0][direction]"))
	}
}

func TestBuildListQuery_AdvancedSortOverridesSimple(t *testing.T) {
	inputs := []*core.Connection{
		conn("sort_field", core.ConnectionTypeString, "Name"),
		conn("sort", core.ConnectionTypeObject, `[{"field":"Created","direction":"asc"},{"field":"Name"}]`),
	}
	q, err := BuildListQuery(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("sort[0][field]") != "Created" || q.Get("sort[0][direction]") != "asc" {
		t.Errorf("advanced sort not applied: %q/%q", q.Get("sort[0][field]"), q.Get("sort[0][direction]"))
	}
	if q.Get("sort[1][field]") != "Name" {
		t.Errorf("second sort key=%q", q.Get("sort[1][field]"))
	}
}

func TestBuildListQuery_InvalidSortJSON(t *testing.T) {
	inputs := []*core.Connection{conn("sort", core.ConnectionTypeObject, `{"field":"Name"}`)}
	if _, err := BuildListQuery(inputs); err == nil {
		t.Error("expected an error: sort must be a JSON array, not an object")
	}
}

func TestBuildListQuery_ParsedSortArray(t *testing.T) {
	// The engine may deliver an object input already parsed (e.g. wired to an
	// upstream array output) rather than as a JSON string — must not be routed
	// through String() and re-parsed.
	parsed := []interface{}{
		map[string]interface{}{"field": "Created", "direction": "desc"},
		map[string]interface{}{"field": "Name"},
	}
	inputs := []*core.Connection{
		conn("sort_field", core.ConnectionTypeString, "Ignored"),
		conn("sort", core.ConnectionTypeObject, parsed),
	}
	q, err := BuildListQuery(inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Get("sort[0][field]") != "Created" || q.Get("sort[0][direction]") != "desc" {
		t.Errorf("parsed sort not applied: %q/%q", q.Get("sort[0][field]"), q.Get("sort[0][direction]"))
	}
	if q.Get("sort[1][field]") != "Name" {
		t.Errorf("second parsed sort key=%q", q.Get("sort[1][field]"))
	}
	if q.Get("sort[0][field]") == "Ignored" {
		t.Error("advanced sort should override the simple sort_field")
	}
}

func TestBuildListQuery_SortContiguousIndicesSkippingBlank(t *testing.T) {
	parsed := []interface{}{
		map[string]interface{}{"field": ""},     // skipped
		map[string]interface{}{"field": "Name"}, // should become sort[0], not sort[1]
	}
	inputs := []*core.Connection{conn("sort", core.ConnectionTypeObject, parsed)}
	q, err := BuildListQuery(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("sort[0][field]") != "Name" {
		t.Errorf("blank entry should not leave a gap; sort[0][field]=%q", q.Get("sort[0][field]"))
	}
	if q.Get("sort[1][field]") != "" {
		t.Errorf("expected no sort[1]; got %q", q.Get("sort[1][field]"))
	}
}

func TestCheckResponse(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantErr  bool
		contains string
	}{
		{"2xx ok", 200, `{"id":"rec1"}`, false, ""},
		{"structured error", 422, `{"error":{"type":"INVALID_REQUEST_BODY","message":"bad body"}}`, true, "INVALID_REQUEST_BODY"},
		{"string error", 404, `{"error":"NOT_FOUND"}`, true, "NOT_FOUND"},
		{"rate limit", 429, `{}`, true, "rate limit"},
		{"opaque", 500, `oops`, true, "500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckResponse(&APIResponse{StatusCode: tc.status, Body: []byte(tc.body)})
			if tc.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.contains != "" && (err == nil || !strings.Contains(err.Error(), tc.contains)) {
				t.Errorf("error %v should contain %q", err, tc.contains)
			}
		})
	}
}

func TestCSVToList(t *testing.T) {
	got := CSVToList(" a , ,b,c ")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("CSVToList = %v", got)
	}
	if CSVToList("   ") != nil {
		t.Errorf("blank should yield nil")
	}
}

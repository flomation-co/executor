package script_common

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// kvConn builds an input_vars key_value_array connection from a JSON string of
// {key,value} rows (the wire form the editor stores and substitution leaves).
func kvConn(name, json string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeKeyValueArray, Value: json}
}

func TestCoerceInputValue(t *testing.T) {
	RegisterTestingT(t)

	// JSON shapes decode to typed values.
	Expect(coerceInputValue(`[{"iid":1}]`)).To(Equal([]interface{}{map[string]interface{}{"iid": float64(1)}}))
	Expect(coerceInputValue(`{"a":1}`)).To(Equal(map[string]interface{}{"a": float64(1)}))
	Expect(coerceInputValue("42")).To(Equal(float64(42)))
	Expect(coerceInputValue("true")).To(Equal(true))
	Expect(coerceInputValue(`"quoted"`)).To(Equal("quoted"))

	// Non-JSON stays a raw string; empty stays empty.
	Expect(coerceInputValue("hello world")).To(Equal("hello world"))
	Expect(coerceInputValue("2026-07-28")).To(Equal("2026-07-28"))
	Expect(coerceInputValue("")).To(Equal(""))
	Expect(coerceInputValue("  ")).To(Equal(""))
}

func TestBuildScriptInputs_NamedRowsTypedAndFootgunFixed(t *testing.T) {
	RegisterTestingT(t)

	// The exact scenario that bit the user: a bare array wired into inputs_data
	// yields nothing (non-object), but the SAME array named as an input_vars row
	// arrives TYPED under that name.
	arr := `[{"iid":1,"title":"Draft"}]`
	inputs := []*core.Connection{
		{Name: "inputs_data", Type: core.ConnectionTypeObject, Value: arr}, // bare array — legacy path drops it
		kvConn("input_vars", `[{"key":"merge_requests","value":"[{\"iid\":1,\"title\":\"Draft\"}]"}]`),
	}
	got := BuildScriptInputs(inputs)
	Expect(got).To(HaveKey("merge_requests"))
	Expect(got["merge_requests"]).To(Equal([]interface{}{map[string]interface{}{"iid": float64(1), "title": "Draft"}}))
}

func TestBuildScriptInputs_LegacyObjectFallback(t *testing.T) {
	RegisterTestingT(t)

	// A legacy inputs_data OBJECT (native map, as wired from upstream) still works
	// with no input_vars present.
	inputs := []*core.Connection{
		{Name: "inputs_data", Type: core.ConnectionTypeObject, Value: map[string]interface{}{"a": float64(1), "b": "x"}},
	}
	got := BuildScriptInputs(inputs)
	Expect(got).To(Equal(map[string]interface{}{"a": float64(1), "b": "x"}))
}

func TestBuildScriptInputs_RowsOverlayObjectAndSkipBlankKeys(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "inputs_data", Type: core.ConnectionTypeObject, Value: `{"a":"from_object","c":"kept"}`},
		// row "a" overrides the object's "a"; a blank-keyed row is ignored.
		kvConn("input_vars", `[{"key":"a","value":"from_row"},{"key":"","value":"ignored"},{"key":"n","value":"7"}]`),
	}
	got := BuildScriptInputs(inputs)
	Expect(got["a"]).To(Equal("from_row"))   // named row wins
	Expect(got["c"]).To(Equal("kept"))       // object-only key preserved
	Expect(got["n"]).To(Equal(float64(7)))   // typed
	Expect(got).NotTo(HaveKey(""))           // blank key skipped
}

func TestBuildScriptInputs_Empty(t *testing.T) {
	RegisterTestingT(t)
	Expect(BuildScriptInputs(nil)).To(Equal(map[string]interface{}{}))
}

package core

// Accessor contracts for Connection.
//
// Both accessors are reached with a value whose dynamic type the action layer
// cannot predict: the editor stores literals, the flow engine auto-wires parent
// outputs, and the substitution pass rewrites any input holding a ${...}
// reference into a plain string before an action ever sees it. These tests pin
// what each accessor does with every shape that actually arrives.

import "testing"

func boolConn(v interface{}) *Connection {
	return &Connection{Name: "flag", Type: ConnectionTypeBoolean, Value: v}
}

func intConn(v interface{}) *Connection {
	return &Connection{Name: "limit", Type: ConnectionTypeInteger, Value: v}
}

func TestBooleanReadsLiteralsAndVariableReferences(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  *bool
	}{
		{"literal true", true, ptr(true)},
		{"literal false", false, ptr(false)},

		// The editor's BooleanProperty stores a bound variable as a string, and
		// the substitution pass resolves it to text before Execute runs. These
		// are the shapes that actually reach an action.
		{"resolved variable true", "true", ptr(true)},
		{"resolved variable false", "false", ptr(false)},
		{"resolved variable True", "True", ptr(true)},
		{"resolved variable 1", "1", ptr(true)},
		{"resolved variable 0", "0", ptr(false)},
		{"resolved variable yes", "yes", ptr(true)},
		{"resolved variable no", "no", ptr(false)},
		{"resolved variable on", "on", ptr(true)},
		{"resolved variable off", "off", ptr(false)},
		{"whitespace tolerated", "  true  ", ptr(true)},

		// An unresolvable ${var.missing} is replaced with the empty string, and a
		// never-touched checkbox is nil. Both must read as unset, so that an
		// action gating a destructive operation on this fails closed.
		{"unresolved variable", "", nil},
		{"unset", nil, nil},
		{"garbage", "maybe", nil},
		{"literal leftover", "${var.missing}", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := boolConn(tc.value).Boolean()
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("Boolean(%#v) = %v, want nil", tc.value, *got)
			case tc.want != nil && got == nil:
				t.Fatalf("Boolean(%#v) = nil, want %v", tc.value, *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Fatalf("Boolean(%#v) = %v, want %v", tc.value, *got, *tc.want)
			}
		})
	}
}

func TestBooleanRejectsOtherTypes(t *testing.T) {
	c := &Connection{Name: "flag", Type: ConnectionTypeString, Value: true}
	if got := c.Boolean(); got != nil {
		t.Fatalf("Boolean() on a string-typed input = %v, want nil", *got)
	}
	if got := (*Connection)(nil).Boolean(); got != nil {
		t.Fatalf("Boolean() on a nil connection = %v, want nil", *got)
	}
}

// A nil or bool value used to reach an unchecked c.Value.(string) assertion and
// panic, killing the flow run rather than reading as unset.
func TestNumberDoesNotPanicOnUnexpectedTypes(t *testing.T) {
	for _, value := range []interface{}{nil, true, []string{"x"}, map[string]int{"a": 1}} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Number() panicked on %#v: %v", value, r)
				}
			}()
			if got := intConn(value).Number(); got != nil {
				t.Fatalf("Number(%#v) = %v, want nil", value, *got)
			}
		}()
	}
}

func TestNumberReadsEveryArrivingShape(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  *int64
	}{
		{"int64", int64(7), ptrI(7)},
		{"int", 7, ptrI(7)},
		{"float64 from JSON", float64(7), ptrI(7)},
		{"resolved variable", "7", ptrI(7)},
		{"whitespace tolerated", " 7 ", ptrI(7)},
		{"negative", "-3", ptrI(-3)},
		{"zero", float64(0), ptrI(0)},

		{"unresolved variable", "", nil},
		{"not a number", "seven", nil},
		{"unset", nil, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := intConn(tc.value).Number()
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("Number(%#v) = %v, want nil", tc.value, *got)
			case tc.want != nil && got == nil:
				t.Fatalf("Number(%#v) = nil, want %v", tc.value, *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Fatalf("Number(%#v) = %v, want %v", tc.value, *got, *tc.want)
			}
		})
	}
}

func ptr(b bool) *bool    { return &b }
func ptrI(n int64) *int64 { return &n }

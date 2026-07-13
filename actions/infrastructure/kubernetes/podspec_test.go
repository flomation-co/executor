package kubernetes

import (
	"reflect"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   \t\n", nil},
		{"simple", "echo hello", []string{"echo", "hello"}},
		{"collapses runs of spaces", "  ls   -la  /tmp ", []string{"ls", "-la", "/tmp"}},
		{"tabs and newlines separate", "a\tb\nc", []string{"a", "b", "c"}},
		{"single quotes keep spaces", "sh -c 'echo hi there'", []string{"sh", "-c", "echo hi there"}},
		{"double quotes keep spaces", `sh -c "echo hi there"`, []string{"sh", "-c", "echo hi there"}},
		{"empty single-quoted arg", "a '' b", []string{"a", "", "b"}},
		{"empty double-quoted arg", `a "" b`, []string{"a", "", "b"}},
		{"adjacent quotes concatenate", `foo'bar'"baz"`, []string{"foobarbaz"}},
		{"backslash escapes space", `a\ b`, []string{"a b"}},
		{"single quotes are literal for dollar", `'$HOME'`, []string{"$HOME"}},
		{"double quotes pass dollar through", `"$HOME"`, []string{"$HOME"}},
		{"backslash in double quotes escapes dollar", `"\$x"`, []string{"$x"}},
		{"backslash in double quotes is otherwise literal", `"a\b"`, []string{`a\b`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SplitCommand(tc.in)
			if err != nil {
				t.Fatalf("SplitCommand(%q) unexpected error: %v", tc.in, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("SplitCommand(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitCommandErrors(t *testing.T) {
	for _, in := range []string{
		`echo "unterminated`,
		"echo 'unterminated",
		`trailing\`,
	} {
		if _, err := SplitCommand(in); err == nil {
			t.Fatalf("SplitCommand(%q) expected an error, got nil", in)
		}
	}
}

func TestEnvList(t *testing.T) {
	if got := EnvList(nil); got != nil {
		t.Fatalf("EnvList(nil) = %#v, want nil", got)
	}
	if got := EnvList(map[string]string{}); got != nil {
		t.Fatalf("EnvList(empty) = %#v, want nil", got)
	}

	// Keys must come back sorted so the list is deterministic regardless of map
	// iteration order.
	got := EnvList(map[string]string{"ZED": "3", "ALPHA": "1", "MID": "2"})
	want := []any{
		map[string]interface{}{"name": "ALPHA", "value": "1"},
		map[string]interface{}{"name": "MID", "value": "2"},
		map[string]interface{}{"name": "ZED", "value": "3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EnvList = %#v, want %#v", got, want)
	}
}

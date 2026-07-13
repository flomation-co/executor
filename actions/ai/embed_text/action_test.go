package ai_embed_text

import (
	"testing"

	core "flomation.app/automate/executor"
)

// parseTexts must accept every shape the Texts input arrives in: a real slice
// (straight from an upstream action), a []any of strings (after a JSON round
// trip through the flow store), and a JSON-array string (after ${...}
// substitution rewrites the reference into resolved text). A non-string element
// is a wiring mistake worth naming.
func TestParseTexts(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    []string
		wantErr bool
	}{
		{"nil", nil, nil, false},
		{"empty string", "", nil, false},
		{"empty json array", "[]", nil, false},
		{"real []string", []string{"a", "b"}, []string{"a", "b"}, false},
		{"[]any of strings", []any{"a", "b"}, []string{"a", "b"}, false},
		{"json array string", `["first", "second"]`, []string{"first", "second"}, false},
		{"[]any with a non-string", []any{"a", 3}, nil, true},
		{"malformed json string", `["a",`, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var c *core.Connection
			if tc.value != nil {
				c = &core.Connection{Name: "texts", Type: core.ConnectionTypeObject, Value: tc.value}
			}
			got, err := parseTexts(c)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// readTexts resolves the single Text and the Texts batch, and reports when a
// non-empty batch overrode a Text the operator also filled in.
func TestReadTexts(t *testing.T) {
	str := func(name, v string) *core.Connection {
		return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: v}
	}
	batch := func(v interface{}) *core.Connection {
		return &core.Connection{Name: "texts", Type: core.ConnectionTypeObject, Value: v}
	}

	t.Run("single text only", func(t *testing.T) {
		texts, ignored, err := readTexts([]*core.Connection{str("text", "hello")})
		if err != nil || ignored || len(texts) != 1 || texts[0] != "hello" {
			t.Fatalf("texts=%v ignored=%v err=%v", texts, ignored, err)
		}
	})

	t.Run("batch overrides text and reports it", func(t *testing.T) {
		texts, ignored, err := readTexts([]*core.Connection{
			str("text", "single"),
			batch([]string{"a", "b"}),
		})
		if err != nil || !ignored || len(texts) != 2 {
			t.Fatalf("texts=%v ignored=%v err=%v — batch should win and ignoredSingle should be true", texts, ignored, err)
		}
	})

	t.Run("batch only does not report ignored", func(t *testing.T) {
		_, ignored, err := readTexts([]*core.Connection{batch([]string{"a"})})
		if err != nil || ignored {
			t.Fatalf("ignored=%v err=%v — nothing was overridden", ignored, err)
		}
	})

	t.Run("nothing supplied", func(t *testing.T) {
		texts, _, err := readTexts([]*core.Connection{})
		if err != nil || len(texts) != 0 {
			t.Fatalf("texts=%v err=%v", texts, err)
		}
	})
}

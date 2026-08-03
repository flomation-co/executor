package anthropic

import "testing"

// TestModelRejectsSamplingParams pins which Anthropic models reject the
// temperature/top_p/top_k sampling parameters, so an upgrade to a newer model
// (e.g. Opus 5) never re-triggers the "`temperature` is deprecated for this
// model" 400 the opt-in temperature handling was added to prevent.
func TestModelRejectsSamplingParams(t *testing.T) {
	cases := []struct {
		model  string
		reject bool
	}{
		// Reject sampling params:
		{"claude-opus-5", true},
		{"claude-opus-5-0", true},
		{"claude-opus-5-0-20260601", true},
		{"claude-opus-4-7", true},
		{"claude-opus-4-8", true},
		{"claude-opus-4-9", true},
		{"claude-fable-5", true},
		{"claude-mythos-5", true},
		{"claude-opus-6", true},
		// Still accept sampling params:
		{"claude-opus-4-6", false},
		{"claude-opus-4-6-1m", false},
		{"claude-opus-4-5", false},
		{"claude-opus-4-1-20250805", false},
		{"claude-opus-4-0", false},
		{"claude-sonnet-4-6", false},
		{"claude-haiku-4-5", false},
		{"", false},
	}
	for _, c := range cases {
		if got := modelRejectsSamplingParams(c.model); got != c.reject {
			t.Errorf("modelRejectsSamplingParams(%q) = %v, want %v", c.model, got, c.reject)
		}
	}
}

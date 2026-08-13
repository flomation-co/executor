package ai_common

import "testing"

// TestSuppressResponse guards the empty-message fix: an empty/whitespace final
// response (e.g. a tool loop that hit its round cap mid-tool-call) must be
// suppressed, so the agent never posts a blank message; [NO_RESPONSE] stays
// suppressed; real text is delivered.
func TestSuppressResponse(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"   \n\t ", true},
		{"[NO_RESPONSE]", true},
		{"sure [NO_RESPONSE] not for me", true},
		{"Hello there", false},
		{"  Done.  ", false},
	}
	for _, c := range cases {
		if got := SuppressResponse(c.in); got != c.want {
			t.Errorf("SuppressResponse(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

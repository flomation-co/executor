package openai

import "testing"

// TestOpenAIModelRejectsTemperature pins which OpenAI models reject a
// non-default temperature (the reasoning o-series), so the opt-in temperature
// handling keeps working when an agent is upgraded to one. Crucially, gpt-4o
// must NOT be treated as an o-series model.
func TestOpenAIModelRejectsTemperature(t *testing.T) {
	cases := []struct {
		model  string
		reject bool
	}{
		{"o1", true},
		{"o1-mini", true},
		{"o3", true},
		{"o3-mini", true},
		{"o4-mini", true},
		{"o5", true},
		// Chat families still accept temperature:
		{"gpt-4o", false},
		{"gpt-4o-mini", false},
		{"gpt-4.1", false},
		{"gpt-5", false},
		{"gpt-3.5-turbo", false},
		{"", false},
	}
	for _, c := range cases {
		if got := openAIModelRejectsTemperature(c.model); got != c.reject {
			t.Errorf("openAIModelRejectsTemperature(%q) = %v, want %v", c.model, got, c.reject)
		}
	}
}

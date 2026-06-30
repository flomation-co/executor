package groq

import (
	"os"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// TestIntegrationLiveCompletion exercises the real Groq endpoint end-to-end
// through Execute. It is skipped unless GROQ_API_KEY is set, so it never runs
// in CI without credentials.
func TestIntegrationLiveCompletion(t *testing.T) {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		t.Skip("GROQ_API_KEY not set; skipping live Groq integration test")
	}
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: key},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama-3.3-70b-versatile"},
		{Name: "system_prompt", Type: core.ConnectionTypeText, Value: "You are terse."},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Reply with exactly: Groq works."},
		{Name: "max_tokens", Type: core.ConnectionTypeInteger, Value: float64(40)},
		{Name: "temperature", Type: core.ConnectionTypeString, Value: "0.1"},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(true))
	Expect(result["should_respond"]).To(Equal(true))
	Expect(result["model"]).To(ContainSubstring("llama-3.3-70b"))
	Expect(result["response"]).To(ContainSubstring("Groq works"))
	Expect(result["total_tokens"]).To(BeNumerically(">", int64(0)))
}

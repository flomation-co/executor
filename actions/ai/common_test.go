package ai_common

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestParseConversationHistory_Nil(t *testing.T) {
	RegisterTestingT(t)
	Expect(ParseConversationHistory(nil)).To(BeNil())
}

func TestParseConversationHistory_JSONString(t *testing.T) {
	RegisterTestingT(t)
	raw := `[{"role":"user","content":"hello"},{"role":"assistant","content":"hi"}]`
	got := ParseConversationHistory(raw)
	Expect(got).To(HaveLen(2))
	Expect(got[0].Role).To(Equal("user"))
	Expect(got[0].Content).To(Equal("hello"))
	Expect(got[1].Role).To(Equal("assistant"))
	Expect(got[1].Content).To(Equal("hi"))
}

func TestParseConversationHistory_EmptyString(t *testing.T) {
	RegisterTestingT(t)
	Expect(ParseConversationHistory("")).To(BeNil())
	Expect(ParseConversationHistory("   ")).To(BeNil())
}

func TestParseConversationHistory_MalformedJSON(t *testing.T) {
	RegisterTestingT(t)
	Expect(ParseConversationHistory("not json")).To(BeNil())
}

func TestParseConversationHistory_SliceOfMaps(t *testing.T) {
	RegisterTestingT(t)
	raw := []map[string]string{
		{"role": "user", "content": "a"},
		{"role": "assistant", "content": "b"},
	}
	got := ParseConversationHistory(raw)
	Expect(got).To(HaveLen(2))
	Expect(got[1].Content).To(Equal("b"))
}

func TestParseConversationHistory_InterfaceSlice(t *testing.T) {
	RegisterTestingT(t)
	// Simulates output from json.Unmarshal into interface{}.
	raw := []interface{}{
		map[string]interface{}{"role": "user", "content": "x"},
		map[string]interface{}{"role": "assistant", "content": "y"},
	}
	got := ParseConversationHistory(raw)
	Expect(got).To(HaveLen(2))
	Expect(got[0].Role).To(Equal("user"))
	Expect(got[1].Role).To(Equal("assistant"))
}

func TestTruncateHistoryForBudget_FitsWithoutChange(t *testing.T) {
	RegisterTestingT(t)
	history := []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	got := TruncateHistoryForBudget(history, "sys", "new question", 1000, 200000)
	Expect(got).To(HaveLen(2))
}

func TestTruncateHistoryForBudget_DropsOldestWhenOverBudget(t *testing.T) {
	RegisterTestingT(t)
	// Each message is ~250 tokens (1000 chars) so 10 messages ~= 2500 tokens.
	longContent := make([]byte, 1000)
	for i := range longContent {
		longContent[i] = 'a'
	}
	history := make([]Message, 10)
	for i := range history {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		history[i] = Message{Role: role, Content: string(longContent) + string(rune('0'+i))}
	}
	// Small context window so truncation is forced.
	got := TruncateHistoryForBudget(history, "", "prompt", 500, 2000)
	Expect(len(got)).To(BeNumerically("<", len(history)))
	// Most recent message must be preserved.
	Expect(got[len(got)-1].Content).To(Equal(history[len(history)-1].Content))
}

func TestTruncateHistoryForBudget_OptOutWhenContextZero(t *testing.T) {
	RegisterTestingT(t)
	history := []Message{{Role: "user", Content: "hi"}}
	got := TruncateHistoryForBudget(history, "", "prompt", 100, 0)
	Expect(got).To(HaveLen(1))
}

func TestTruncateHistoryForBudget_PromptAlreadyOverBudget(t *testing.T) {
	RegisterTestingT(t)
	history := []Message{{Role: "user", Content: "older"}}
	// system + max_tokens alone exceed the context window.
	got := TruncateHistoryForBudget(history, "sys", "prompt", 10000, 1000)
	Expect(got).To(BeNil())
}

func TestModelContextWindow_KnownModels(t *testing.T) {
	RegisterTestingT(t)
	Expect(ModelContextWindow("claude-sonnet-4-6")).To(Equal(200000))
	Expect(ModelContextWindow("gpt-4o")).To(Equal(128000))
	Expect(ModelContextWindow("gpt-4.1")).To(Equal(1000000))
	Expect(ModelContextWindow("gpt-3.5-turbo")).To(Equal(16000))
}

func TestModelContextWindow_UnknownModel(t *testing.T) {
	RegisterTestingT(t)
	Expect(ModelContextWindow("some-future-model")).To(Equal(32000))
}

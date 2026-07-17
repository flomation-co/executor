package ai_common

import (
	"regexp"
	"strings"
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

// Azure passes a deployment name, and Azure names cannot contain '.', so its
// GPT-3.5 is spelled "gpt-35-turbo". It must size to the same 16k window as
// the dotted OpenAI id — the 32000 default would exceed the real one and let
// TruncateHistoryForBudget hand Azure a request it rejects.
func TestModelContextWindow_AzureDotlessNames(t *testing.T) {
	RegisterTestingT(t)
	Expect(ModelContextWindow("gpt-35-turbo")).To(Equal(16000))
	Expect(ModelContextWindow("gpt-35-turbo-16k")).To(Equal(16000))
	// Non-Azure ids keep their existing windows: the dotless case must not
	// widen its net over the rest of the switch.
	Expect(ModelContextWindow("gpt-4o")).To(Equal(128000))
	Expect(ModelContextWindow("openai/gpt-4.1-mini")).To(Equal(1000000))
	Expect(ModelContextWindow("chat-prod")).To(Equal(32000))
}

// The window drives the truncation budget, so a dotless Azure deployment must
// actually shed history rather than pass a 28k conversation to a 16k model.
func TestTruncateHistoryForBudget_AzureGPT35IsTruncated(t *testing.T) {
	RegisterTestingT(t)
	var history []Message
	for i := 0; i < 40; i++ {
		history = append(history, Message{Role: "user", Content: strings.Repeat("word ", 700)})
	}
	got := TruncateHistoryForBudget(history, "sys", "prompt", 2048, ModelContextWindow("gpt-35-turbo"))
	Expect(len(got)).To(BeNumerically("<", len(history)), "history was not truncated for a 16k Azure deployment")
}

func TestModelContextWindow_UnknownModel(t *testing.T) {
	RegisterTestingT(t)
	Expect(ModelContextWindow("some-future-model")).To(Equal(32000))
}

// OpenRouter reaches models through provider-prefixed ids
// ("anthropic/claude-sonnet-5"); the substring matching must resolve
// both the families added for OpenRouter and the pre-existing families
// when prefixed.
func TestModelContextWindow_OpenRouterPrefixedModels(t *testing.T) {
	RegisterTestingT(t)
	Expect(ModelContextWindow("openai/gpt-5.4-mini")).To(Equal(400000))
	Expect(ModelContextWindow("anthropic/claude-sonnet-5")).To(Equal(200000))
	Expect(ModelContextWindow("google/gemini-3.1-pro-preview")).To(Equal(1000000))
	Expect(ModelContextWindow("google/gemini-3.1-flash-lite-image")).To(Equal(32000))
	Expect(ModelContextWindow("google/gemini-3-pro-image-preview")).To(Equal(32000))
	Expect(ModelContextWindow("deepseek/deepseek-v3.2")).To(Equal(128000))
	Expect(ModelContextWindow("mistralai/mistral-large-2512")).To(Equal(128000))
	Expect(ModelContextWindow("meta-llama/llama-4-maverick")).To(Equal(128000))
	Expect(ModelContextWindow("x-ai/grok-4-fast")).To(Equal(128000))
	// Prefixed ids of families the switch already knew must not regress.
	Expect(ModelContextWindow("openai/gpt-oss-20b:free")).To(Equal(128000))
	Expect(ModelContextWindow("openai/gpt-4.1")).To(Equal(1000000))
	// Unprefixed local (Ollama) and Groq variants of the same families
	// have far smaller real windows and must keep the conservative
	// default — the deepseek/mistral cases are scoped to the OpenRouter
	// provider-prefixed ids on purpose.
	Expect(ModelContextWindow("mistral:7b-instruct")).To(Equal(32000))
	Expect(ModelContextWindow("mistral-saba-24b")).To(Equal(32000))
	Expect(ModelContextWindow("deepseek-r1:8b")).To(Equal(32000))
	Expect(ModelContextWindow("mistralai/mistral-small-24b-instruct-2501")).To(Equal(32000))
}

// TestBlobTokenInstructions_NoInlineRealLookingHandle catches the
// fundamental bug class: a real-looking example handle in the
// instruction is something Anthropic models will copy verbatim.
// Three failures in production traced to this:
//
//  1. Original example was 16 chars (a3f9c2d1b4e7805f). AI copied
//     that shape. Caught in execution 2611489e.
//  2. MR !151 used a 33-char example (typo extra `3`). AI trimmed
//     to 32 chars to match the prose rule. Caught in 9dcf8bc3.
//  3. The 32-char correct example a3f9c2d1b4e7805f7e9d0c2b1a8e6f4d
//     was ALSO copied verbatim by the AI. Caught in ee749f82.
//
// The fix is to remove any inline real-looking handle from the
// prompt entirely — replace with `<HANDLE>` placeholder text the
// model cannot copy. We pin the absence of any flo:blob:<32-hex>?
// substring in the instructions so a future "let me add a helpful
// concrete example" edit fails the test loudly.
func TestBlobTokenInstructions_NoInlineRealLookingHandle(t *testing.T) {
	RegisterTestingT(t)

	// Match `flo:blob:` followed by 32+ lowercase hex chars. ANY
	// substring that matches is a real-looking handle the AI could
	// copy.
	re := regexp.MustCompile(`flo:blob:[a-f0-9]{16,}`)
	matches := re.FindAllString(BlobTokenInstructions, -1)
	Expect(matches).To(BeEmpty(),
		"instruction must NOT contain any flo:blob:<real-looking-hex-handle> substring — Anthropic models copy them verbatim. Use <HANDLE> placeholder text instead. Found: %v", matches)

	// Pin the prose-only format guidance that replaces the example.
	Expect(BlobTokenInstructions).To(ContainSubstring("32 lowercase hexadecimal"),
		"instruction must state the format requirement in prose")
	Expect(BlobTokenInstructions).To(ContainSubstring("<HANDLE>"),
		"instruction must use <HANDLE> placeholder instead of a copyable example")
	Expect(BlobTokenInstructions).To(ContainSubstring("NEVER invent"),
		"instruction must explicitly forbid invention")
}

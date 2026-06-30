package ai_common

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
)

// Message is the common chat-message shape used by both OpenAI and Anthropic
// (role + content). Used for passing conversation history into AI actions.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ParseConversationHistory normalises the value of a conversation_history
// input into a slice of Messages. The input value may be nil, a JSON string,
// or an already-parsed slice (either []Message, []map[string]interface{},
// []map[string]string, or []interface{}). Unknown shapes return nil.
func ParseConversationHistory(raw interface{}) []Message {
	if raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case []Message:
		return v
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return nil
		}
		var msgs []Message
		if err := json.Unmarshal([]byte(v), &msgs); err == nil {
			return msgs
		}
		return nil
	case []map[string]string:
		out := make([]Message, 0, len(v))
		for _, m := range v {
			out = append(out, Message{Role: m["role"], Content: m["content"]})
		}
		return out
	case []map[string]interface{}:
		out := make([]Message, 0, len(v))
		for _, m := range v {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			out = append(out, Message{Role: role, Content: content})
		}
		return out
	case []interface{}:
		out := make([]Message, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				role, _ := m["role"].(string)
				content, _ := m["content"].(string)
				out = append(out, Message{Role: role, Content: content})
			} else if m, ok := item.(Message); ok {
				out = append(out, m)
			}
		}
		return out
	}

	return nil
}

// approxTokens estimates the token count of a string using a conservative
// characters-per-token heuristic. This avoids a runtime dependency on a real
// tokeniser (tiktoken etc) while giving us a safe upper bound for truncation.
// The ratio of ~4 chars/token is the widely-used approximation for English.
func approxTokens(s string) int {
	return (len(s) + 3) / 4
}

// ApproxTokens is the exported version of approxTokens for use by
// AI action packages that need token estimation.
func ApproxTokens(s string) int {
	return approxTokens(s)
}

// ApproxMessageTokens estimates the token cost of a single message. A small
// overhead is added to account for role tags and message boundaries in the
// underlying API wire format.
func ApproxMessageTokens(m Message) int {
	const messageOverhead = 8
	return approxTokens(m.Role) + approxTokens(m.Content) + messageOverhead
}

// TruncateHistoryForBudget drops the oldest messages from history until the
// total estimated token cost of (history + systemPrompt + userPrompt + buffer
// for the reply) fits within modelContext. The caller supplies:
//
//   - history:       messages ordered oldest → newest
//   - systemPrompt:  system instructions (may be empty)
//   - userPrompt:    the new user turn about to be appended
//   - maxTokens:     max_tokens reserved for the model's reply
//   - modelContext:  the target model's total context window in tokens
//
// Returns the (possibly shortened) history. If modelContext is zero or
// negative, history is returned unchanged — the caller has opted out of
// truncation.
func TruncateHistoryForBudget(history []Message, systemPrompt, userPrompt string, maxTokens, modelContext int) []Message {
	if modelContext <= 0 || len(history) == 0 {
		return history
	}

	fixedCost := approxTokens(systemPrompt) + approxTokens(userPrompt) + maxTokens + 64 // safety margin
	budget := modelContext - fixedCost
	if budget <= 0 {
		// Prompt alone already exceeds budget — nothing we can do with history.
		return nil
	}

	total := 0
	for _, m := range history {
		total += ApproxMessageTokens(m)
	}
	if total <= budget {
		return history
	}

	// Drop from the front (oldest) until we fit.
	drop := 0
	for drop < len(history) && total > budget {
		total -= ApproxMessageTokens(history[drop])
		drop++
	}
	return history[drop:]
}

// ModelContextWindow returns a conservative context-window size (in tokens)
// for a given model identifier. Unknown models fall back to a safe default.
// These numbers are intentionally conservative — the goal is to prevent
// 400/413 errors from the provider, not to maximise utilisation.
func ModelContextWindow(model string) int {
	m := strings.ToLower(model)
	switch {
	// Anthropic Claude
	case strings.Contains(m, "claude-opus-4-6") && strings.Contains(m, "1m"):
		return 1000000
	case strings.Contains(m, "claude-opus-4-6"),
		strings.Contains(m, "claude-sonnet-4-6"),
		strings.Contains(m, "claude-haiku-4-5"):
		return 200000
	case strings.Contains(m, "claude-"):
		return 200000
	// OpenAI
	case strings.Contains(m, "gpt-4.1"):
		return 1000000
	case strings.Contains(m, "gpt-4o"), strings.Contains(m, "o3"), strings.Contains(m, "o4"):
		return 128000
	case strings.Contains(m, "gpt-4"):
		return 128000
	case strings.Contains(m, "gpt-3.5"):
		return 16000
	// Groq-hosted open models. Groq publishes large context windows but
	// caps the practical maximum per deployment; these are conservative
	// figures matched to Groq's current production limits.
	case strings.Contains(m, "gpt-oss"),
		strings.Contains(m, "llama-4-scout"),
		strings.Contains(m, "llama-3.3-70b"),
		strings.Contains(m, "llama-3.1-8b"),
		strings.Contains(m, "qwen/qwen3"),
		strings.Contains(m, "groq/compound"):
		return 128000
	}
	return 32000 // safe default
}

// BlobTokenInstructions is appended to the system prompt of any AI
// action that exposes tools. It teaches the model how to handle the
// flo:blob:<handle> references the executor emits when a tool
// produces a large output (audio, image, video, OCR text, etc.).
//
// Without this instruction the model frequently invents a placeholder
// string for the argument (the canonical failure: "generated_audio_base64"
// in execution 0bab2c40-3905-463c-9103-dc164d381f69). With it, the
// model passes the verbatim token from the prior tool result, the
// executor resolves the token into the original bytes, and the
// downstream tool receives the real value without ever touching the
// context window.
const BlobTokenInstructions = `

LARGE TOOL OUTPUTS — some tools store large media (audio, image,
video, document bytes) outside your context window. When that
happens, the tool's result includes an "Outputs available as
references:" section listing each off-loaded field next to an
opaque handle. The handle is a string in this exact shape:

  flo:blob:<HANDLE>?size=<N>&type=<MIME>

The <HANDLE> portion is exactly 32 lowercase hexadecimal characters
[0-9a-f] with NO hyphens, NO underscores, NO uppercase. It is a
random opaque identifier generated by the executor — you have NO
way to generate one yourself.

WHEN A REAL HANDLE IS LISTED in "Outputs available as references:"
this turn — paste the ENTIRE token (prefix + handle + size + type)
verbatim into the downstream tool's argument. Do not summarise,
paraphrase, or describe the contents in place of the token.

WHEN A FIELD IS LISTED UNDER "No blob token is available for the
following outputs" — the off-load failed for that field this turn.
You have NO handle for it. Do NOT substitute another handle you
remember from elsewhere; the data is unreachable. Either retry the
producing tool, or fall back to text.

WHEN YOU SEE NEITHER section in your context — no off-loaded media
is available. Do NOT pass a flo:blob: argument to any tool — there
is no real handle for you to pass.

NEVER invent or guess a handle. NEVER copy a handle-shaped string
from earlier in your prompt or memory and use it as a token. The
executor rejects any handle it didn't issue this turn, and a
guessed handle ALWAYS fails the downstream tool.
`

// AppendBlobTokenInstructions returns the supplied system prompt
// with the blob-token guidance suffixed. Idempotent — calling it
// twice doesn't duplicate the block.
func AppendBlobTokenInstructions(systemPrompt string) string {
	if strings.Contains(systemPrompt, "LARGE TOOL OUTPUTS —") {
		return systemPrompt
	}
	return systemPrompt + BlobTokenInstructions
}

// === Vision-block promotion (M2.5) ===
//
// When an inbound message carries image attachments (Telegram photo,
// Slack file_share with image mime, etc.), the API's auto-promote
// step writes `[attached: name (mime, size) → flo:blob:…]` markers
// into the user message text. That's enough for the model to KNOW an
// image exists, but not enough for it to SEE the image — so without
// further work, vision-capable models hallucinate descriptions of
// whatever they think the user might have sent.
//
// ExtractVisionBlobs scans a prompt for image attachment markers,
// resolves each blob token to its bytes via the per-execution
// BlobStore, and returns the (stripped-text, ordered-image-list)
// pair ready for vendor-specific assembly into a multi-part content
// array. Non-image attachments (audio, video, document) are LEFT in
// the text — we don't have a generic vision-block analogue and the
// marker still helps the model reason about them.

// VisionBlob is a resolved image attachment ready for vendor-specific
// block assembly. Mime is the canonical content-type the AI vendor
// needs (image/jpeg, image/png, etc.); Bytes is the raw decoded blob.
type VisionBlob struct {
	Name  string
	Mime  string
	Bytes []byte
}

// BlobFetcher is the narrow interface ExtractVisionBlobs needs.
// Satisfied by *core.BlobStore. Used as a parameter so this package
// doesn't have to import core (would be a cycle).
type BlobFetcher interface {
	Get(token string) ([]byte, error)
}

// attachedMarkerPattern matches one full [attached: …] line. The
// `→` separator (U+2192) is the load-bearing marker — we use it
// instead of `->` so user text containing the literal string
// `attached:` doesn't false-match. Captures: 1=name, 2=mime, 3=token.
var attachedMarkerPattern = regexp.MustCompile(
	`\[attached: ([^()\[\]]+?) \(([^,]+?), [^)]+?\) → (flo:blob:[0-9a-f]{32}(?:\?[^\]]*)?)\]`)

// ExtractVisionBlobs scans `prompt` for image attachment markers,
// resolves each to bytes via `fetcher.Get`, and returns:
//
//   - stripped: the prompt with every successfully-resolved image
//     marker removed (the image moves to the dedicated block).
//   - images: the resolved image blobs in marker-appearance order.
//
// Non-image markers (mime not starting with "image/") are LEFT in
// the text untouched. Markers whose blob fetch FAILS are also left
// in the text — better the model sees a stale reference than
// silently loses awareness an attachment exists.
//
// Returns (prompt, nil) unchanged when no markers match.
func ExtractVisionBlobs(prompt string, fetcher BlobFetcher) (stripped string, images []VisionBlob) {
	if fetcher == nil {
		return prompt, nil
	}
	matches := attachedMarkerPattern.FindAllStringSubmatchIndex(prompt, -1)
	if len(matches) == 0 {
		return prompt, nil
	}

	stripped = prompt
	// Walk matches in reverse so byte-offset splices stay valid as
	// we shrink the string. Collected images get reversed back to
	// forward order at the end.
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		name := prompt[m[2]:m[3]]
		mime := prompt[m[4]:m[5]]
		token := prompt[m[6]:m[7]]

		if !strings.HasPrefix(mime, "image/") {
			continue
		}
		bytes, err := fetcher.Get(token)
		if err != nil || len(bytes) == 0 {
			continue
		}
		images = append(images, VisionBlob{
			Name:  name,
			Mime:  mime,
			Bytes: bytes,
		})

		// Splice the marker out, eating one neighbouring newline so
		// the surrounding text reads naturally.
		start, end := m[0], m[1]
		if start > 0 && stripped[start-1] == '\n' {
			start--
		} else if end < len(stripped) && stripped[end] == '\n' {
			end++
		}
		stripped = stripped[:start] + stripped[end:]
	}

	// Reverse images to forward (visual-attachment) order.
	for i, j := 0, len(images)-1; i < j; i, j = i+1, j-1 {
		images[i], images[j] = images[j], images[i]
	}

	stripped = strings.TrimSpace(stripped)
	stripped = regexp.MustCompile(`\n{3,}`).ReplaceAllString(stripped, "\n\n")
	return stripped, images
}

// BuildAnthropicUserContent returns the content value for an
// Anthropic user message — either the prompt string verbatim (no
// images) or a content-block array of text + image blocks.
//
// Anthropic's image block shape:
//
//	{"type": "image", "source": {"type": "base64", "media_type": "image/jpeg", "data": "<b64>"}}
func BuildAnthropicUserContent(prompt string, fetcher BlobFetcher) interface{} {
	stripped, images := ExtractVisionBlobs(prompt, fetcher)
	if len(images) == 0 {
		return prompt
	}
	blocks := make([]map[string]interface{}, 0, len(images)+1)
	// Images first — Claude documentation recommends placing images
	// before the accompanying text for best comprehension.
	for _, img := range images {
		blocks = append(blocks, map[string]interface{}{
			"type": "image",
			"source": map[string]interface{}{
				"type":       "base64",
				"media_type": img.Mime,
				"data":       base64.StdEncoding.EncodeToString(img.Bytes),
			},
		})
	}
	if strings.TrimSpace(stripped) != "" {
		blocks = append(blocks, map[string]interface{}{
			"type": "text",
			"text": stripped,
		})
	}
	return blocks
}

// BuildOpenAIUserContent does the same for OpenAI's vision API.
//
// OpenAI's image block shape uses a data: URL:
//
//	{"type": "image_url", "image_url": {"url": "data:image/jpeg;base64,<b64>"}}
func BuildOpenAIUserContent(prompt string, fetcher BlobFetcher) interface{} {
	stripped, images := ExtractVisionBlobs(prompt, fetcher)
	if len(images) == 0 {
		return prompt
	}
	blocks := make([]map[string]interface{}, 0, len(images)+1)
	if strings.TrimSpace(stripped) != "" {
		blocks = append(blocks, map[string]interface{}{
			"type": "text",
			"text": stripped,
		})
	}
	for _, img := range images {
		blocks = append(blocks, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": "data:" + img.Mime + ";base64," + base64.StdEncoding.EncodeToString(img.Bytes),
			},
		})
	}
	return blocks
}

package core

// Tests for DetokeniseInputs — the LLM tool-call resolution path. The
// failure modes covered here are the exact shapes the agent loop in
// flow.go relies on to surface clear errors to the AI:
//
//   * nil store + token-shaped value → error WITH the field name AND
//     the value left in place (so a strict caller that ignores the
//     error and passes it to an action gets a confusing base64 error
//     downstream — which is precisely why the agent loop now short-
//     circuits on the error rather than running the action anyway).
//
//   * Non-token values pass through unchanged regardless of store
//     availability.
//
//   * Multiple failed tokens collapse to ONE error so the agent loop
//     surfaces a single actionable message to the AI instead of a
//     wall of repeated text.

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// TestDetokeniseInputs_NilStore_ReturnsErrorForTokenValues is the load-
// bearing regression test for the f.blobs vs f.Blobs() race fix. With
// store=nil and a token-shaped value, DetokeniseInputs must return an
// error so the agent loop short-circuits the tool call. Before the fix,
// the engine logged a warning and ran the action anyway — which is
// what produced the "illegal base64 data at input byte 3" failure mode
// in execution 589116a7.
func TestDetokeniseInputs_NilStore_ReturnsErrorForTokenValues(t *testing.T) {
	RegisterTestingT(t)

	args := map[string]interface{}{
		"audio_base64": "flo:blob:f0f0e5c8b2a14d9e8c7f3b1a6d2e4f5c?size=443080&type=audio/mpeg",
	}
	out, err := DetokeniseInputs(args, nil)

	Expect(err).To(HaveOccurred(), "nil store with a token must surface an error so the agent loop can react")
	Expect(err.Error()).To(ContainSubstring("audio_base64"), "error must name the offending field for AI feedback")
	// Token is left in place — the caller is expected NOT to pass it
	// to an action (that would just produce a base64 decode error).
	Expect(out["audio_base64"]).To(Equal(args["audio_base64"]))
}

// TestDetokeniseInputs_NonTokenValues_PassThrough confirms the function
// is a no-op for any value that isn't a blob token. Strings that happen
// to start with "flo:" but don't match the full token shape, integers,
// nil values, and short strings all flow through untouched.
func TestDetokeniseInputs_NonTokenValues_PassThrough(t *testing.T) {
	RegisterTestingT(t)

	args := map[string]interface{}{
		"plain":        "hello",
		"empty":        "",
		"looks-likey":  "flo:not-a-blob",
		"short":        "flo:blob:tooshort",
		"int":          42,
		"nil":          nil,
		"actual-text":  "the user said 'flo:blob:something'",
	}
	out, err := DetokeniseInputs(args, nil)

	Expect(err).NotTo(HaveOccurred(), "no real tokens → no error even with nil store")
	for k, v := range args {
		if v == nil {
			Expect(out[k]).To(BeNil(), "%s should pass through unchanged (nil)", k)
			continue
		}
		Expect(out[k]).To(Equal(v), "%s should pass through unchanged", k)
	}
}

// TestDetokeniseInputs_EmptyMap is the trivial edge case — early return
// path. Worth pinning because a regression that called store methods
// even on empty input would crash with the nil store + would surface
// in production as "blob token resolution failed; no blob store
// available" warnings for tools the AI never even passed args to.
func TestDetokeniseInputs_EmptyMap(t *testing.T) {
	RegisterTestingT(t)
	out, err := DetokeniseInputs(map[string]interface{}{}, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(out).To(BeEmpty())
}

// TestDetokeniseInputs_MultipleFailedTokens_SingleError keeps the
// error message tractable. The agent loop surfaces this string to the
// AI as a tool result; a wall of "field1: error; field2: error;
// field3: error" wastes context tokens. The contract is "name the
// first failure clearly" — once the AI fixes that, subsequent failures
// surface on the next turn if any remain.
func TestDetokeniseInputs_MultipleFailedTokens_SingleError(t *testing.T) {
	RegisterTestingT(t)

	args := map[string]interface{}{
		"audio_base64": "flo:blob:f0f0e5c8b2a14d9e8c7f3b1a6d2e4f5c?size=1&type=audio/mpeg",
		"image_base64": "flo:blob:abababababababababababababababab?size=1&type=image/png",
	}
	_, err := DetokeniseInputs(args, nil)

	Expect(err).To(HaveOccurred())
	// Exactly ONE field name appears — the implementation records the
	// "first" error; map iteration order is non-deterministic so we
	// don't pin WHICH field, only that the count is exactly one.
	parts := strings.Count(err.Error(), ": blob token received but no blob store available")
	Expect(parts).To(Equal(1), "agent loop expects a single concise error, not one per field")
}

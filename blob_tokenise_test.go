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
	out, err := DetokeniseInputs(args, nil, nil)

	Expect(err).To(HaveOccurred(), "nil store with a token must surface an error so the agent loop can react")
	Expect(err.Error()).To(ContainSubstring("audio_base64"), "error must name the offending field for AI feedback")
	// Token is left in place — the caller is expected NOT to pass it
	// to an action (that would just produce a base64 decode error).
	Expect(out["audio_base64"]).To(Equal(args["audio_base64"]))
}

// TestDetokeniseInputs_NonTokenValues_PassThrough confirms the function
// is a no-op for any value that genuinely isn't a blob reference.
// Strings that start with "flo:" but DON'T include the "blob:" prefix,
// integers, nil values, and the literal "flo:blob:" appearing inside a
// surrounding sentence are all left alone. Strings that START with
// "flo:blob:" but are malformed get caught by the malformed-token
// branch (covered by the UUID + NearMissPrefix tests above).
func TestDetokeniseInputs_NonTokenValues_PassThrough(t *testing.T) {
	RegisterTestingT(t)

	args := map[string]interface{}{
		"plain":       "hello",
		"empty":       "",
		"looks-likey": "flo:not-a-blob",
		"int":         42,
		"nil":         nil,
		"actual-text": "the user said 'flo:blob:something'",
	}
	out, err := DetokeniseInputs(args, nil, nil)

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
	out, err := DetokeniseInputs(map[string]interface{}{}, nil, nil)
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
	_, err := DetokeniseInputs(args, nil, nil)

	Expect(err).To(HaveOccurred())
	// Exactly ONE field name appears — the implementation records the
	// "first" error; map iteration order is non-deterministic so we
	// don't pin WHICH field, only that the count is exactly one.
	parts := strings.Count(err.Error(), ": blob token received but no blob store available")
	Expect(parts).To(Equal(1), "agent loop expects a single concise error, not one per field")
}

// TestDetokeniseInputs_UUIDFormatToken_RejectedAsMalformed locks the
// exact hallucination pattern seen in execution 9fe18d8a:
// `flo:blob:3b3b8e0e-7f3a-4c3a-a7d5-0e1e6b3b8e0e?size=513715&type=audio/mpeg`.
// Anthropic models reflexively format invented handles as UUIDs with
// hyphens, which fails the strict 32-lowercase-hex handle check. Before
// this fix DetokeniseInputs silently passed the malformed token to the
// action — which emitted "illegal base64 data at input byte 3", a
// useless message the AI couldn't act on. Now we name the format
// requirement explicitly so the AI can correct itself on the next
// turn (or, better, recognise that it shouldn't have invented the
// token in the first place).
func TestDetokeniseInputs_UUIDFormatToken_RejectedAsMalformed(t *testing.T) {
	RegisterTestingT(t)

	args := map[string]interface{}{
		"audio_base64": "flo:blob:3b3b8e0e-7f3a-4c3a-a7d5-0e1e6b3b8e0e?size=513715&type=audio/mpeg",
	}
	out, err := DetokeniseInputs(args, nil, nil)

	Expect(err).To(HaveOccurred(), "UUID-format token must be rejected — it's a hallucination")
	Expect(err.Error()).To(ContainSubstring("audio_base64"))
	Expect(err.Error()).To(ContainSubstring("32 lowercase hex"),
		"error must name the format requirement so the AI can correct itself")
	// Token is left in place but the engine's short-circuit (see
	// flow.go) will catch the error and skip the action.
	Expect(out["audio_base64"]).To(Equal(args["audio_base64"]))
}

// TestDetokeniseInputs_NearMissPrefix_RejectedAsMalformed catches the
// broader class of "looks like a token but isn't" — anything starting
// with the flo:blob: prefix that doesn't parse as a valid handle.
// Covers shorter-than-32-chars handles, longer-than-32-chars, mixed
// case, non-hex characters.
func TestDetokeniseInputs_NearMissPrefix_RejectedAsMalformed(t *testing.T) {
	RegisterTestingT(t)

	cases := []string{
		"flo:blob:short",                              // too short
		"flo:blob:f0f0e5c8b2a14d9e8c7f3b1a6d2e4f5",    // 31 chars, off-by-one
		"flo:blob:F0F0E5C8B2A14D9E8C7F3B1A6D2E4F5C",   // uppercase
		"flo:blob:G0G0e5c8b2a14d9e8c7f3b1a6d2e4f5c",   // non-hex char
		"flo:blob:f0f0e5c8b2a14d9e8c7f3b1a6d2e4f5cXX", // 34 chars, too long
	}
	for _, malformed := range cases {
		args := map[string]interface{}{"audio_base64": malformed}
		_, err := DetokeniseInputs(args, nil, nil)
		Expect(err).To(HaveOccurred(),
			"malformed token %q should be rejected", malformed)
		Expect(err.Error()).To(ContainSubstring("not a valid blob token"),
			"error for %q should explain why", malformed)
	}
}

package telegram_common

// Tests for the shared helpers consumed by all four M4 outbound
// file actions. We focus on the pure-function logic
// (ResolveFileBytes, ValidateChannelID) — that's where the
// behavioural decisions live. SendMultipartFile is straight-line
// net/http multipart writing; its correctness is observable through
// integration testing once the actions are wired into a live
// Telegram bot, and unit-testing it would require constructing a
// real *core.Flow which is more setup than this helper warrants.

import (
	"encoding/base64"
	"testing"

	. "github.com/onsi/gomega"
)

func TestValidateChannelID_Resolved_NoError(t *testing.T) {
	RegisterTestingT(t)
	Expect(ValidateChannelID("12345")).To(Succeed())
	Expect(ValidateChannelID("@somechannel")).To(Succeed())
	Expect(ValidateChannelID("-1001234567890")).To(Succeed())
}

func TestValidateChannelID_UnresolvedTemplate_Errors(t *testing.T) {
	// The classic ${flow.channel_id} leaking through unresolved when
	// the context didn't populate the field. Without this check the
	// literal string would land as the chat_id in the multipart body
	// and Telegram would reject with an opaque chat_not_found error.
	RegisterTestingT(t)
	err := ValidateChannelID("${flow.channel_id}")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("unresolved template variable"))

	err2 := ValidateChannelID("#{flow.channel_id}")
	Expect(err2).To(HaveOccurred())
}

func TestResolveFileBytes_Base64Standard(t *testing.T) {
	RegisterTestingT(t)
	original := []byte("hello world")
	enc := base64.StdEncoding.EncodeToString(original)

	got, err := ResolveFileBytes(nil, "", enc)
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(original))
}

func TestResolveFileBytes_Base64URLSafe(t *testing.T) {
	// Some upstreams (browser-side, AI tool args that round-tripped
	// through JSON URL params) emit URL-safe base64. We must accept
	// both alphabets — same behaviour send_voice has had since
	// migration 36.
	RegisterTestingT(t)
	original := []byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb}
	enc := base64.URLEncoding.EncodeToString(original)

	got, err := ResolveFileBytes(nil, "", enc)
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(original))
}

func TestResolveFileBytes_NeitherInput_ReturnsErrNoFile(t *testing.T) {
	RegisterTestingT(t)
	_, err := ResolveFileBytes(nil, "", "")
	Expect(err).To(MatchError(ErrNoFile))
}

func TestResolveFileBytes_FileBlobAsRawBytes(t *testing.T) {
	// When the AI tool loop's DetokeniseInputs has already resolved
	// the blob token to raw bytes (as a Go string holding arbitrary
	// bytes), file_blob is no longer a flo:blob: token — it's the
	// content itself. ResolveFileBytes must pass it through as-is
	// without trying to base64-decode.
	RegisterTestingT(t)
	rawBytes := []byte("raw image bytes here \x89PNG")
	got, err := ResolveFileBytes(nil, string(rawBytes), "")
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(rawBytes))
}

func TestResolveFileBytes_BlobTokenPreferredOverBase64(t *testing.T) {
	// When both inputs are supplied, file_blob wins. The wrapper
	// actions pass both through in the order (file_blob, file_base64)
	// so resolution order is fixed here.
	RegisterTestingT(t)
	rawBytes := []byte("from blob path")
	other := base64.StdEncoding.EncodeToString([]byte("from base64 path"))

	got, err := ResolveFileBytes(nil, string(rawBytes), other)
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(rawBytes))
}

func TestResolveFileBytes_InvalidBase64_Errors(t *testing.T) {
	RegisterTestingT(t)
	_, err := ResolveFileBytes(nil, "", "###not-base64-at-all###")
	Expect(err).To(MatchError(ContainSubstring("decode file_base64")))
}

package slack_file_upload

// Tests for the M5 resolveFileBytes helper — the load-bearing logic
// that picks which input "wins" when an AI/user wires more than one.
// The actual Slack HTTP round-trip is left to integration testing
// (it's the same 3-step external-upload flow that already worked
// pre-M5; M5 only changes how `contentBytes` gets populated).

import (
	"encoding/base64"
	"testing"

	. "github.com/onsi/gomega"
)

func TestResolveFileBytes_TextContentFallback(t *testing.T) {
	// Pre-M5 behaviour: a flow that wires only `content` (text) keeps
	// working unchanged. This is the backwards-compatibility check.
	RegisterTestingT(t)
	got, _, err := resolveFileBytes(nil, "", "", "hello CSV row 1\nrow 2")
	Expect(err).NotTo(HaveOccurred())
	Expect(string(got)).To(Equal("hello CSV row 1\nrow 2"))
}

func TestResolveFileBytes_Base64Standard(t *testing.T) {
	RegisterTestingT(t)
	original := []byte("file contents")
	got, _, err := resolveFileBytes(nil, "", base64.StdEncoding.EncodeToString(original), "")
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(original))
}

func TestResolveFileBytes_Base64URLSafe(t *testing.T) {
	RegisterTestingT(t)
	original := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG magic bytes
	got, _, err := resolveFileBytes(nil, "", base64.URLEncoding.EncodeToString(original), "")
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(original))
}

func TestResolveFileBytes_FileBlobAsRawBytes(t *testing.T) {
	// When DetokeniseInputs in the tool loop has already resolved the
	// blob token to raw bytes (as a Go string holding arbitrary
	// bytes), file_blob is no longer a flo:blob: token but the actual
	// content. The helper must pass it through verbatim.
	RegisterTestingT(t)
	rawBytes := []byte("raw image bytes \x89PNG\r\n")
	got, _, err := resolveFileBytes(nil, string(rawBytes), "", "")
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(rawBytes))
}

func TestResolveFileBytes_FileBlobWinsOverBase64(t *testing.T) {
	RegisterTestingT(t)
	winner := []byte("from file_blob path")
	loser := base64.StdEncoding.EncodeToString([]byte("from file_base64 path"))

	got, _, err := resolveFileBytes(nil, string(winner), loser, "")
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(winner))
}

func TestResolveFileBytes_FileBase64WinsOverContent(t *testing.T) {
	// When file_blob is absent but both file_base64 and content are
	// supplied, file_base64 wins. The contract is "binary > text",
	// matching the priority order in the package doc.
	RegisterTestingT(t)
	winner := []byte{0x01, 0x02, 0x03}
	got, _, err := resolveFileBytes(nil, "", base64.StdEncoding.EncodeToString(winner), "should be ignored")
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(winner))
}

func TestResolveFileBytes_AllEmpty_Errors(t *testing.T) {
	RegisterTestingT(t)
	_, _, err := resolveFileBytes(nil, "", "", "")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("one of file_blob"))
}

func TestResolveFileBytes_InvalidBase64_Errors(t *testing.T) {
	RegisterTestingT(t)
	_, _, err := resolveFileBytes(nil, "", "###definitely-not-base64###", "")
	Expect(err).To(MatchError(ContainSubstring("decode file_base64")))
}

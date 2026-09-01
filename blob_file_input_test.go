package core

// Regression tests for the download → upload handoff.
//
// An agent chains "Download File" into a media upload by passing the
// flo:blob: token from one tool result into the next tool call. The
// receiving input is declared ConnectionTypeFile, whose documented
// contract (see the constant in flow.go) is that the token arrives
// VERBATIM so the action can resolve it with ResolveToLocalFile —
// which is the only path that knows the blob's MIME type.
//
// DetokeniseInputs used to substitute the raw bytes for EVERY blob
// token in the argument map, file-typed or not. The action then took
// the "unknown representation" branch of ResolveToLocalFile and wrote
// the bytes to a scratch file with NO extension, because the MIME hint
// only ever lived on the token that had just been thrown away. Meta's
// /adimages rejects an upload whose filename has no image extension,
// which is what "the blob handoff between download → upload needs
// fixing" looked like from the agent's side.
//
// Note the size dependence, which is what made this look intermittent:
// EmitMediaFile returns a flo:blob: token at or below 25 MB and a
// flo:file: reference above it. flo:file: was never detokenised, so
// large images worked and ordinary ad images did not.

import (
	"net/http"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

// jpegBytes is a minimal JPEG: SOI + APP0/JFIF + EOI. Enough for
// http.DetectContentType to report image/jpeg, and deliberately NOT
// valid base64 so decodeMaybeBase64 leaves it alone.
var jpegBytes = []byte{
	0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00,
	0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
}

const testHandle = "a1b2c3d4e5f60718293a4b5c6d7e8f90"

func testBlobToken() string {
	return BlobTokenPrefix + testHandle + "?size=22&type=image%2Fjpeg"
}

// seededBlobStore returns a store whose Get resolves testHandle from the
// in-process cache, so no HTTP is attempted.
func seededBlobStore(t *testing.T) *BlobStore {
	t.Helper()
	s := NewBlobStore(http.DefaultClient, "http://api.invalid", "org-1", "", "exec-1")
	s.cache[testHandle] = append([]byte(nil), jpegBytes...)
	return s
}

// TestDetokeniseInputs_FileTypedInput_KeepsTokenVerbatim is the core
// regression: a file-typed input must receive the token, not the bytes.
func TestDetokeniseInputs_FileTypedInput_KeepsTokenVerbatim(t *testing.T) {
	RegisterTestingT(t)

	token := testBlobToken()
	args := map[string]interface{}{"image": token}

	out, err := DetokeniseInputs(args, seededBlobStore(t), map[string]bool{"image": true})

	Expect(err).ToNot(HaveOccurred())
	Expect(out["image"]).To(Equal(token),
		"a ConnectionTypeFile input must keep the token so the action can resolve its MIME type")
}

// TestDetokeniseInputs_FileTypedInput_StillRejectsUnknownToken confirms
// that keeping the token verbatim does NOT weaken the hallucination
// guard: the handle is still proven to resolve before the action runs,
// so a made-up handle fails with a message naming the field rather than
// surfacing later as an opaque fetch error.
func TestDetokeniseInputs_FileTypedInput_StillRejectsUnknownToken(t *testing.T) {
	RegisterTestingT(t)

	unknown := BlobTokenPrefix + "ffffffffffffffffffffffffffffffff?size=1&type=image%2Fpng"
	args := map[string]interface{}{"image": unknown}

	_, err := DetokeniseInputs(args, seededBlobStore(t), map[string]bool{"image": true})

	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("image"))
}

// TestDetokeniseInputs_NonFileInput_StillResolvesToBytes guards the
// legacy path: audio_base64 and friends were handed the base64 STRING
// originally, and must keep receiving it.
func TestDetokeniseInputs_NonFileInput_StillResolvesToBytes(t *testing.T) {
	RegisterTestingT(t)

	args := map[string]interface{}{"audio_base64": testBlobToken()}

	out, err := DetokeniseInputs(args, seededBlobStore(t), map[string]bool{"image": true})

	Expect(err).ToNot(HaveOccurred())
	Expect(out["audio_base64"]).To(Equal(string(jpegBytes)),
		"inputs that are not file-typed keep the existing detokenise-to-value behaviour")
}

// TestResolveToLocalFile_TokenCarriesExtension_RawBytesDoNot documents
// WHY the token has to survive: only the token carries the MIME hint,
// and the extension is what the upload sends to Meta as the filename.
func TestResolveToLocalFile_TokenCarriesExtension_RawBytesDoNot(t *testing.T) {
	RegisterTestingT(t)
	chdirWorkspace(t)

	f := &Flow{}
	f.blobs = seededBlobStore(t)

	viaToken, mimeType, err := f.ResolveToLocalFile(testBlobToken())
	Expect(err).ToNot(HaveOccurred())
	Expect(filepath.Ext(viaToken)).To(Equal(".jpg"))
	Expect(mimeType).To(Equal("image/jpeg"))

	// The pre-fix path: the engine had already replaced the token with
	// the bytes, so the resolver had nothing to infer a name from.
	viaBytes, _, err := f.ResolveToLocalFile(string(jpegBytes))
	Expect(err).ToNot(HaveOccurred())
	Expect(filepath.Ext(viaBytes)).To(BeEmpty(),
		"raw bytes carry no MIME hint — this is the extensionless filename Meta rejected")
}

// TestResolveToLocalFile_URLIsRejected covers the other half of the
// report. Passing an http(s) URL used to succeed: the URL TEXT was
// written to a scratch file and uploaded as though it were the image,
// so the failure surfaced as an unhelpful complaint from Meta about the
// file contents. URLs are deliberately not fetched here (that is the
// SSRF-guarded Download File action's job), so say so plainly.
func TestResolveToLocalFile_URLIsRejected(t *testing.T) {
	RegisterTestingT(t)
	chdirWorkspace(t)

	f := &Flow{}

	for _, u := range []string{
		"https://cdn.example.com/creative.jpg",
		"http://cdn.example.com/creative.jpg",
		"  https://cdn.example.com/creative.jpg  ",
	} {
		_, _, err := f.ResolveToLocalFile(u)
		Expect(err).To(HaveOccurred(), "a URL is not file content: %q", u)
		Expect(err.Error()).To(ContainSubstring("Download File"),
			"the error must name the action that turns a URL into a file reference")
	}
}

// TestExtForMime_CanonicalAndStable pins the extensions that end up as
// upload filenames. mime.ExtensionsByType alone is host-dependent —
// image/jpeg resolves to ".jfif" on macOS — so a flow would produce a
// different filename depending on which runner picked it up.
func TestExtForMime_CanonicalAndStable(t *testing.T) {
	RegisterTestingT(t)

	Expect(extForMime("image/jpeg")).To(Equal(".jpg"))
	Expect(extForMime("image/png")).To(Equal(".png"))
	Expect(extForMime("video/mp4")).To(Equal(".mp4"))
	Expect(extForMime("audio/mpeg")).To(Equal(".mp3"))

	// Parameters and casing are tolerated, since a Content-Type header
	// is where these MIME strings usually come from.
	Expect(extForMime("image/jpeg; charset=binary")).To(Equal(".jpg"))
	Expect(extForMime("IMAGE/PNG")).To(Equal(".png"))

	Expect(extForMime("")).To(BeEmpty())
}

// TestFileRefInputNames_SelectsOnlyFileTypedInputs covers the lookup the
// agent loop builds from the matched tool node.
func TestFileRefInputNames_SelectsOnlyFileTypedInputs(t *testing.T) {
	RegisterTestingT(t)

	names := FileRefInputNames([]*Connection{
		{Name: "image", Type: ConnectionTypeFile},
		{Name: "account_id", Type: ConnectionTypeString},
		{Name: "access_token", Type: ConnectionTypeSecret},
		nil,
		{Name: "attachment", Type: ConnectionTypeFile},
	})

	Expect(names).To(Equal(map[string]bool{"image": true, "attachment": true}))
	Expect(FileRefInputNames(nil)).To(BeNil())
}

// TestResolveToBytes_URLIsRejected mirrors the ResolveToLocalFile guard
// for the sink-action seam: a Slack/Drive upload handed a URL used to
// upload a file whose contents were the URL text.
func TestResolveToBytes_URLIsRejected(t *testing.T) {
	RegisterTestingT(t)
	chdirWorkspace(t)

	f := &Flow{}
	_, _, err := f.ResolveToBytes("https://cdn.example.com/creative.jpg")

	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Download File"))
}

// TestResolveToBytes_NonURLValuesUnaffected keeps the guard narrow: it
// must not swallow base64, plain text, or text that merely contains a URL.
func TestResolveToBytes_NonURLValuesUnaffected(t *testing.T) {
	RegisterTestingT(t)
	chdirWorkspace(t)

	f := &Flow{}
	for _, v := range []string{
		"see https://example.com for details",
		"ftp://example.com/file.jpg",
		"https://",
		"just some text",
	} {
		_, _, err := f.ResolveToBytes(v)
		Expect(err).ToNot(HaveOccurred(), "value must pass through: %q", v)
	}
}

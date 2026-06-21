package ai_common

// Tests for the vision-block promotion path (ExtractVisionBlobs +
// BuildAnthropicUserContent + BuildOpenAIUserContent).
//
// The tests use an in-process stubBlobFetcher rather than a real
// BlobStore — the vision logic is purely about marker parsing,
// resolution, and vendor-specific formatting; the blob store's
// network round-trip is unrelated.

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// stubBlobFetcher returns bytes keyed by token. A token mapped to nil
// returns ErrNotFound so we can exercise the "fetch failed → leave
// marker in place" branch.
type stubBlobFetcher struct {
	store map[string][]byte
}

var errNotFound = errors.New("not found")

func (s *stubBlobFetcher) Get(token string) ([]byte, error) {
	b, ok := s.store[token]
	if !ok || b == nil {
		return nil, errNotFound
	}
	return b, nil
}

// makeAttachedMarker builds the canonical attachment marker the API
// renderer emits. Centralised so the assertions don't drift from the
// real format if it evolves.
func makeAttachedMarker(name, mime, size, token string) string {
	return "[attached: " + name + " (" + mime + ", " + size + ") → " + token + "]"
}

func TestExtractVisionBlobs_NoMarkers_ReturnsUnchanged(t *testing.T) {
	RegisterTestingT(t)
	stripped, images := ExtractVisionBlobs("just a normal message", &stubBlobFetcher{})
	Expect(stripped).To(Equal("just a normal message"))
	Expect(images).To(BeNil())
}

func TestExtractVisionBlobs_SingleImageMarker_ResolvedAndStripped(t *testing.T) {
	RegisterTestingT(t)
	token := "flo:blob:0123456789abcdef0123456789abcdef?size=100&type=image%2Fjpeg"
	prompt := "look at this\n\n" + makeAttachedMarker("cat.jpg", "image/jpeg", "100 B", token)
	fetcher := &stubBlobFetcher{store: map[string][]byte{token: []byte("JPEG-bytes")}}

	stripped, images := ExtractVisionBlobs(prompt, fetcher)

	Expect(images).To(HaveLen(1))
	Expect(images[0].Name).To(Equal("cat.jpg"))
	Expect(images[0].Mime).To(Equal("image/jpeg"))
	Expect(images[0].Bytes).To(Equal([]byte("JPEG-bytes")))

	// Marker removed; preceding text retained.
	Expect(stripped).To(Equal("look at this"))
}

func TestExtractVisionBlobs_NonImageMarker_LeftInText(t *testing.T) {
	RegisterTestingT(t)
	token := "flo:blob:0123456789abcdef0123456789abcdef?size=1000&type=application%2Fpdf"
	prompt := "see attached\n\n" + makeAttachedMarker("doc.pdf", "application/pdf", "1.0 KB", token)
	fetcher := &stubBlobFetcher{store: map[string][]byte{token: []byte("PDF-bytes")}}

	stripped, images := ExtractVisionBlobs(prompt, fetcher)

	Expect(images).To(BeEmpty())
	// PDF marker stays — model can't see it visually either way.
	Expect(stripped).To(ContainSubstring("[attached: doc.pdf"))
}

func TestExtractVisionBlobs_MultipleImages_PreservesOrder(t *testing.T) {
	RegisterTestingT(t)
	t1 := "flo:blob:11111111111111111111111111111111?size=10&type=image%2Fpng"
	t2 := "flo:blob:22222222222222222222222222222222?size=10&type=image%2Fjpeg"
	prompt := "look\n\n" + makeAttachedMarker("a.png", "image/png", "10 B", t1) +
		"\n" + makeAttachedMarker("b.jpg", "image/jpeg", "10 B", t2)
	fetcher := &stubBlobFetcher{store: map[string][]byte{
		t1: []byte("PNG"),
		t2: []byte("JPG"),
	}}

	_, images := ExtractVisionBlobs(prompt, fetcher)
	Expect(images).To(HaveLen(2))
	Expect(images[0].Name).To(Equal("a.png"))
	Expect(images[1].Name).To(Equal("b.jpg"))
}

func TestExtractVisionBlobs_FailedFetch_LeavesMarker(t *testing.T) {
	RegisterTestingT(t)
	token := "flo:blob:99999999999999999999999999999999?size=10&type=image%2Fpng"
	prompt := "see this\n\n" + makeAttachedMarker("ghost.png", "image/png", "10 B", token)
	fetcher := &stubBlobFetcher{store: map[string][]byte{}}

	stripped, images := ExtractVisionBlobs(prompt, fetcher)
	Expect(images).To(BeEmpty())
	// Marker retained — the model at least knows the attachment existed.
	Expect(stripped).To(ContainSubstring("[attached: ghost.png"))
}

func TestExtractVisionBlobs_NilFetcher_NoOp(t *testing.T) {
	RegisterTestingT(t)
	token := "flo:blob:0123456789abcdef0123456789abcdef?size=10&type=image%2Fpng"
	prompt := "see this " + makeAttachedMarker("x.png", "image/png", "10 B", token)
	stripped, images := ExtractVisionBlobs(prompt, nil)
	Expect(stripped).To(Equal(prompt))
	Expect(images).To(BeNil())
}

func TestBuildAnthropicUserContent_NoImages_ReturnsString(t *testing.T) {
	RegisterTestingT(t)
	out := BuildAnthropicUserContent("plain text", &stubBlobFetcher{})
	Expect(out).To(BeAssignableToTypeOf(""))
	Expect(out).To(Equal("plain text"))
}

func TestBuildAnthropicUserContent_WithImage_ReturnsBlocks(t *testing.T) {
	RegisterTestingT(t)
	token := "flo:blob:0123456789abcdef0123456789abcdef?size=100&type=image%2Fjpeg"
	prompt := "what is this?\n\n" + makeAttachedMarker("photo.jpg", "image/jpeg", "100 B", token)
	fetcher := &stubBlobFetcher{store: map[string][]byte{token: []byte("PHOTO-BYTES")}}

	out := BuildAnthropicUserContent(prompt, fetcher)
	blocks, ok := out.([]map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(blocks).To(HaveLen(2))

	// Anthropic: images first, then text.
	Expect(blocks[0]["type"]).To(Equal("image"))
	src, _ := blocks[0]["source"].(map[string]interface{})
	Expect(src["type"]).To(Equal("base64"))
	Expect(src["media_type"]).To(Equal("image/jpeg"))
	Expect(src["data"]).To(Equal(base64.StdEncoding.EncodeToString([]byte("PHOTO-BYTES"))))

	Expect(blocks[1]["type"]).To(Equal("text"))
	Expect(blocks[1]["text"]).To(Equal("what is this?"))
}

func TestBuildAnthropicUserContent_ImageOnlyMessage_NoTextBlock(t *testing.T) {
	RegisterTestingT(t)
	token := "flo:blob:0123456789abcdef0123456789abcdef?size=10&type=image%2Fpng"
	prompt := makeAttachedMarker("p.png", "image/png", "10 B", token)
	fetcher := &stubBlobFetcher{store: map[string][]byte{token: []byte("X")}}

	out := BuildAnthropicUserContent(prompt, fetcher)
	blocks, ok := out.([]map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(blocks).To(HaveLen(1))
	Expect(blocks[0]["type"]).To(Equal("image"))
}

func TestBuildOpenAIUserContent_WithImage_ReturnsDataURLBlock(t *testing.T) {
	RegisterTestingT(t)
	token := "flo:blob:0123456789abcdef0123456789abcdef?size=100&type=image%2Fjpeg"
	prompt := "describe this\n\n" + makeAttachedMarker("photo.jpg", "image/jpeg", "100 B", token)
	fetcher := &stubBlobFetcher{store: map[string][]byte{token: []byte("DATA")}}

	out := BuildOpenAIUserContent(prompt, fetcher)
	blocks, ok := out.([]map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(blocks).To(HaveLen(2))

	// OpenAI: text first, then image (the ordering matters less for
	// OpenAI than Claude but worth pinning down so a future reordering
	// is intentional).
	Expect(blocks[0]["type"]).To(Equal("text"))
	Expect(blocks[1]["type"]).To(Equal("image_url"))
	url, _ := blocks[1]["image_url"].(map[string]interface{})
	Expect(url["url"]).To(HavePrefix("data:image/jpeg;base64,"))
	Expect(url["url"]).To(ContainSubstring(base64.StdEncoding.EncodeToString([]byte("DATA"))))
}

func TestExtractVisionBlobs_MixedImageAndDoc_OnlyImageExtracted(t *testing.T) {
	RegisterTestingT(t)
	tImg := "flo:blob:11111111111111111111111111111111?size=10&type=image%2Fjpeg"
	tDoc := "flo:blob:22222222222222222222222222222222?size=10&type=application%2Fpdf"
	prompt := "hi\n\n" +
		makeAttachedMarker("img.jpg", "image/jpeg", "10 B", tImg) + "\n" +
		makeAttachedMarker("doc.pdf", "application/pdf", "10 B", tDoc)
	fetcher := &stubBlobFetcher{store: map[string][]byte{
		tImg: []byte("IMG"),
		tDoc: []byte("DOC"),
	}}

	stripped, images := ExtractVisionBlobs(prompt, fetcher)
	Expect(images).To(HaveLen(1))
	Expect(images[0].Mime).To(Equal("image/jpeg"))
	// Image marker stripped, doc marker retained.
	Expect(stripped).NotTo(ContainSubstring("img.jpg"))
	Expect(stripped).To(ContainSubstring("doc.pdf"))
}

func TestExtractVisionBlobs_SimilarLookingTextNotMatched(t *testing.T) {
	// Sanity check that the regex doesn't false-match on user text
	// resembling an attachment marker. The arrow character is the
	// load-bearing discriminator (U+2192, not "->").
	RegisterTestingT(t)
	prompt := "[attached: cat.jpg (image/jpeg, 100 B) -> flo:blob:0123]"
	stripped, images := ExtractVisionBlobs(prompt, &stubBlobFetcher{})
	Expect(images).To(BeEmpty())
	Expect(stripped).To(Equal(prompt))
	Expect(strings.Contains(prompt, "->")).To(BeTrue())
}

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestBlobStore_PutGetRoundTrip(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	store := NewBlobStore(dir)

	payload := []byte(strings.Repeat("X", 50000))
	token, err := store.Put(payload, "application/octet-stream")
	Expect(err).NotTo(HaveOccurred())
	Expect(token).To(HavePrefix(BlobTokenPrefix))
	Expect(token).To(ContainSubstring("size=50000"))
	Expect(token).To(ContainSubstring("type="))

	got, err := store.Get(token)
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(payload))
}

func TestBlobStore_ParseBlobToken_Shapes(t *testing.T) {
	RegisterTestingT(t)

	type tc struct {
		name   string
		input  string
		ok     bool
		handle string
		size   int
		mime   string
	}
	cases := []tc{
		{"full form", "flo:blob:0123456789abcdef?size=1024&type=audio%2Fmpeg",
			true, "0123456789abcdef", 1024, "audio/mpeg"},
		{"without type", "flo:blob:fedcba9876543210?size=42",
			true, "fedcba9876543210", 42, ""},
		{"bare handle", "flo:blob:0011223344556677",
			true, "0011223344556677", 0, ""},
		{"wrong prefix", "blob:0123456789abcdef",
			false, "", 0, ""},
		{"short handle", "flo:blob:0123",
			false, "", 0, ""},
		{"non-hex handle", "flo:blob:0123456789xxxxxx",
			false, "", 0, ""},
		{"empty", "", false, "", 0, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			RegisterTestingT(t)
			h, s, m, ok := ParseBlobToken(c.input)
			Expect(ok).To(Equal(c.ok))
			if c.ok {
				Expect(h).To(Equal(c.handle))
				Expect(s).To(Equal(c.size))
				Expect(m).To(Equal(c.mime))
			}
		})
	}
}

func TestBlobStore_GetUnknownHandle(t *testing.T) {
	RegisterTestingT(t)

	store := NewBlobStore(t.TempDir())
	_, err := store.Get("flo:blob:0000000000000000?size=0")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("blob not found"))
}

func TestBlobStore_GetNonToken(t *testing.T) {
	RegisterTestingT(t)

	store := NewBlobStore(t.TempDir())
	_, err := store.Get("just a plain string")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("not a blob token"))
}

func TestBlobStore_Cleanup(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	store := NewBlobStore(dir)
	_, err := store.Put([]byte("hello"), "")
	Expect(err).NotTo(HaveOccurred())

	blobDir := filepath.Join(dir, "blobs")
	entries, err := os.ReadDir(blobDir)
	Expect(err).NotTo(HaveOccurred())
	Expect(entries).NotTo(BeEmpty())

	Expect(store.Cleanup()).To(Succeed())

	_, statErr := os.Stat(blobDir)
	Expect(os.IsNotExist(statErr)).To(BeTrue(), "blob dir should be gone after Cleanup")
}

func TestTokeniseLargeOutputs_AppliesThresholdAndKeyHeuristic(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	store := NewBlobStore(dir)

	bigBase64 := strings.Repeat("A", BlobThresholdBytes+1)
	bigProse := strings.Repeat("This is a long story. ", 1500)

	outputs := map[string]interface{}{
		"audio_base64":     bigBase64,
		"tool_result":      "Generated audio successfully",
		"audio_size_bytes": 415077,
		"narrative":        bigProse,        // not media-shaped key
		"small_audio_data": "short content", // below threshold
	}

	manifest := TokeniseLargeOutputs(outputs, store)
	Expect(manifest).To(HaveLen(1), "only audio_base64 should be off-loaded")
	Expect(manifest[0].Field).To(Equal("audio_base64"))
	Expect(manifest[0].Size).To(Equal(len(bigBase64)))
	Expect(manifest[0].Token).To(HavePrefix(BlobTokenPrefix))

	// Outputs map left untouched — DB / inspector / graph wiring
	// still see the real values.
	Expect(outputs["audio_base64"]).To(Equal(bigBase64))
}

func TestTokeniseLargeOutputs_UsesAudioFormatHint(t *testing.T) {
	RegisterTestingT(t)

	store := NewBlobStore(t.TempDir())
	outputs := map[string]interface{}{
		"audio_base64": strings.Repeat("Z", BlobThresholdBytes+1),
		"audio_format": "ogg_opus_48000",
	}
	manifest := TokeniseLargeOutputs(outputs, store)
	Expect(manifest).To(HaveLen(1))
	Expect(manifest[0].Mime).To(Equal("audio/ogg"),
		"OGG format hint should override the default audio/mpeg suffix mapping")
}

func TestDetokeniseInputs_ResolvesKnownToken(t *testing.T) {
	RegisterTestingT(t)

	store := NewBlobStore(t.TempDir())
	payload := []byte(strings.Repeat("R", 20000))
	token, err := store.Put(payload, "audio/mpeg")
	Expect(err).NotTo(HaveOccurred())

	args := map[string]interface{}{
		"audio_base64": token,
		"caption":      "literal user string",
	}
	resolved, derr := DetokeniseInputs(args, store)
	Expect(derr).NotTo(HaveOccurred())
	Expect(resolved["audio_base64"]).To(Equal(string(payload)))
	Expect(resolved["caption"]).To(Equal("literal user string"),
		"non-token values must pass through unchanged")
}

func TestDetokeniseInputs_UnknownTokenSurfacesError(t *testing.T) {
	RegisterTestingT(t)

	store := NewBlobStore(t.TempDir())
	args := map[string]interface{}{
		"audio_base64": "flo:blob:deadbeefdeadbeef?size=99",
	}
	resolved, derr := DetokeniseInputs(args, store)
	Expect(derr).To(HaveOccurred())
	Expect(derr.Error()).To(ContainSubstring("audio_base64"))
	Expect(derr.Error()).To(ContainSubstring("blob not found"))
	// On error we keep the original value so the action sees
	// SOMETHING — the failing decode then surfaces a clearer
	// error to the LLM than silent success would.
	Expect(resolved["audio_base64"]).To(ContainSubstring("flo:blob:"))
}

func TestFormatTokenManifest_Empty(t *testing.T) {
	RegisterTestingT(t)
	Expect(FormatTokenManifest(nil)).To(Equal(""))
	Expect(FormatTokenManifest([]TokenManifestEntry{})).To(Equal(""))
}

func TestFormatTokenManifest_RendersEntries(t *testing.T) {
	RegisterTestingT(t)
	out := FormatTokenManifest([]TokenManifestEntry{
		{Field: "audio_base64", Token: "flo:blob:abc?size=1", Size: 1, Mime: "audio/mpeg"},
		{Field: "image_data", Token: "flo:blob:def?size=2", Size: 2, Mime: "image/png"},
	})
	Expect(out).To(ContainSubstring("Outputs available as references"))
	Expect(out).To(ContainSubstring("audio_base64: flo:blob:abc?size=1"))
	Expect(out).To(ContainSubstring("image_data: flo:blob:def?size=2"))
}

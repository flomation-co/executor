package core

// Tests for the filename that travels with a file: how it is sanitised, how
// it survives the blob/file reference chain, and what an upload falls back to
// when nothing carried a name.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestSanitiseFilename(t *testing.T) {
	RegisterTestingT(t)

	// Ordinary names survive untouched, spaces and unicode included.
	Expect(SanitiseFilename("report.pdf")).To(Equal("report.pdf"))
	Expect(SanitiseFilename("Q3 résumé (final).docx")).To(Equal("Q3 résumé (final).docx"))
	Expect(SanitiseFilename("  spaced.png  ")).To(Equal("spaced.png"))

	// A name must never express a path. These are the shapes that reach us
	// from Content-Disposition headers and LLM tool arguments.
	Expect(SanitiseFilename("../../etc/passwd")).To(Equal("passwd"))
	Expect(SanitiseFilename("/etc/passwd")).To(Equal("passwd"))
	Expect(SanitiseFilename(`C:\Windows\system32\evil.exe`)).To(Equal("evil.exe"))
	Expect(SanitiseFilename("dir/sub/pic.png")).To(Equal("pic.png"))
	Expect(SanitiseFilename("a\x00b.png")).To(Equal("ab.png"))
	Expect(SanitiseFilename("no:colons*or?globs.txt")).To(Equal("nocolonsorglobs.txt"))

	// Nothing usable left.
	Expect(SanitiseFilename("")).To(BeEmpty())
	Expect(SanitiseFilename("   ")).To(BeEmpty())
	Expect(SanitiseFilename("..")).To(BeEmpty())
	Expect(SanitiseFilename("/")).To(BeEmpty())
	Expect(SanitiseFilename(".hidden")).To(Equal("hidden"))
}

func TestSanitiseFilename_TruncatesKeepingExtension(t *testing.T) {
	RegisterTestingT(t)

	long := strings.Repeat("a", 400) + ".png"
	got := SanitiseFilename(long)

	Expect(len(got)).To(BeNumerically("<=", maxFilenameBytes))
	Expect(filepath.Ext(got)).To(Equal(".png"),
		"the extension decides how the receiving service handles the upload, so it must survive truncation")
}

func TestEnsureFilenameExtension(t *testing.T) {
	RegisterTestingT(t)

	Expect(EnsureFilenameExtension("photo", "image/jpeg")).To(Equal("photo.jpg"))
	// An explicit extension is never second-guessed.
	Expect(EnsureFilenameExtension("notes.log", "text/plain")).To(Equal("notes.log"))
	// Nothing to add, nothing to add it to.
	Expect(EnsureFilenameExtension("photo", "")).To(Equal("photo"))
	Expect(EnsureFilenameExtension("", "image/png")).To(BeEmpty())
}

func TestBlobTokenName_RoundTrip(t *testing.T) {
	RegisterTestingT(t)

	base := BlobTokenPrefix + testHandle + "?size=22&type=image%2Fjpeg"
	Expect(BlobTokenName(base)).To(BeEmpty())

	named := WithBlobTokenName(base, "quarterly report.jpg")
	Expect(IsBlobToken(named)).To(BeTrue(), "adding a name must not break the token")
	Expect(BlobTokenName(named)).To(Equal("quarterly report.jpg"))

	// size and type still parse, and keep their canonical order.
	h, size, mime, ok := ParseBlobToken(named)
	Expect(ok).To(BeTrue())
	Expect(h).To(Equal(testHandle))
	Expect(size).To(Equal(22))
	Expect(mime).To(Equal("image/jpeg"))
	Expect(named).To(HavePrefix(base), "existing parameters keep their order so tokens stay stable")

	// Replacing a name does not stack parameters.
	again := WithBlobTokenName(named, "other.jpg")
	Expect(BlobTokenName(again)).To(Equal("other.jpg"))
	Expect(strings.Count(again, "name=")).To(Equal(1))
}

func TestBlobTokenName_RejectsPathsAndNonTokens(t *testing.T) {
	RegisterTestingT(t)

	base := BlobTokenPrefix + testHandle + "?size=1"

	// A token is untrusted input: a name that expresses a path is reduced to
	// its last component on the way in AND on the way out.
	hostile := WithBlobTokenName(base, "../../../etc/passwd")
	Expect(BlobTokenName(hostile)).To(Equal("passwd"))

	// Hand-crafted token carrying a traversal directly.
	Expect(BlobTokenName(BlobTokenPrefix + testHandle + "?size=1&name=..%2F..%2Fpasswd")).
		To(Equal("passwd"))

	// Not a token, or nothing worth keeping: unchanged.
	Expect(WithBlobTokenName("not-a-token", "x.png")).To(Equal("not-a-token"))
	Expect(WithBlobTokenName(base, "   ")).To(Equal(base))
	Expect(BlobTokenName("plain text")).To(BeEmpty())
}

func TestFilenameForRef(t *testing.T) {
	RegisterTestingT(t)

	named := WithBlobTokenName(BlobTokenPrefix+testHandle+"?size=1&type=image%2Fpng", "logo.png")
	Expect(FilenameForRef(named)).To(Equal("logo.png"))
	Expect(FilenameForRef(BlobTokenPrefix + testHandle)).To(BeEmpty())

	Expect(FilenameForRef("flo:file:.flomation/media/ab12/invoice.pdf")).To(Equal("invoice.pdf"))
	Expect(FilenameForRef("some base64 or text")).To(BeEmpty())
}

func TestUploadFilename_Priority(t *testing.T) {
	RegisterTestingT(t)

	ref := WithBlobTokenName(BlobTokenPrefix+testHandle+"?size=1&type=image%2Fpng", "carried.png")

	// 1. What was asked for wins.
	Expect(UploadFilename("chosen.png", ref, "image/png", "upload")).To(Equal("chosen.png"))
	// An explicit name still gains a missing extension.
	Expect(UploadFilename("chosen", ref, "image/png", "upload")).To(Equal("chosen.png"))
	// And is still sanitised.
	Expect(UploadFilename("../../chosen.png", ref, "image/png", "upload")).To(Equal("chosen.png"))

	// 2. Otherwise the name the reference carried.
	Expect(UploadFilename("", ref, "image/png", "upload")).To(Equal("carried.png"))

	// 3. Otherwise a unique name with the right extension.
	gen := UploadFilename("", "raw bytes", "image/png", "advert")
	Expect(gen).To(HavePrefix("advert-"))
	Expect(filepath.Ext(gen)).To(Equal(".png"))
	Expect(gen).ToNot(Equal(UploadFilename("", "raw bytes", "image/png", "advert")),
		"two uploads in one execution must not collide")

	// Never empty, even with nothing to go on.
	Expect(UploadFilename("", "", "", "")).ToNot(BeEmpty())
}

func TestMediaScratchFileNamed_KeepsNameAndStaysInWorkspace(t *testing.T) {
	RegisterTestingT(t)
	ws := chdirWorkspace(t)

	f := &Flow{}
	p, err := f.MediaScratchFileNamed("annual report.pdf")
	Expect(err).ToNot(HaveOccurred())
	Expect(filepath.Base(p)).To(Equal("annual report.pdf"))

	rel, err := filepath.Rel(ws, p)
	Expect(err).ToNot(HaveOccurred())
	Expect(rel).ToNot(HavePrefix(".."), "scratch files must stay inside the workspace")

	// Two calls with the SAME name must not collide: the directory is what
	// carries the uniqueness.
	q, err := f.MediaScratchFileNamed("annual report.pdf")
	Expect(err).ToNot(HaveOccurred())
	Expect(q).ToNot(Equal(p))

	// A hostile name cannot climb out.
	esc, err := f.MediaScratchFileNamed("../../../../etc/passwd")
	Expect(err).ToNot(HaveOccurred())
	rel, err = filepath.Rel(ws, esc)
	Expect(err).ToNot(HaveOccurred())
	Expect(rel).ToNot(HavePrefix(".."))
	Expect(filepath.Base(esc)).To(Equal("passwd"))
}

func TestResolveToLocalFile_UsesTokenNameForScratchFile(t *testing.T) {
	RegisterTestingT(t)
	chdirWorkspace(t)

	f := &Flow{}
	f.blobs = seededBlobStore(t)

	named := WithBlobTokenName(testBlobToken(), "hero shot.jpg")
	path, mimeType, err := f.ResolveToLocalFile(named)

	Expect(err).ToNot(HaveOccurred())
	Expect(filepath.Base(path)).To(Equal("hero shot.jpg"),
		"an action that names its upload from the resolved path must get the real name")
	Expect(mimeType).To(Equal("image/jpeg"))

	b, err := os.ReadFile(path)
	Expect(err).ToNot(HaveOccurred())
	Expect(b).To(Equal(jpegBytes))
}

func TestEmitMediaFile_StampsNameOntoToken(t *testing.T) {
	RegisterTestingT(t)
	chdirWorkspace(t)

	f := &Flow{}
	f.blobs = seededBlobStore(t)

	// No blob backend reachable here, so this falls back to a flo:file:
	// reference — whose path already carries the name.
	p, err := f.MediaScratchFileNamed("statement.pdf")
	Expect(err).ToNot(HaveOccurred())
	Expect(os.WriteFile(p, []byte("%PDF-1.4"), 0o600)).To(Succeed())

	ref, err := f.EmitMediaFile(p)
	Expect(err).ToNot(HaveOccurred())
	Expect(FilenameForRef(ref)).To(Equal("statement.pdf"))
}

func TestUploadDestination(t *testing.T) {
	RegisterTestingT(t)

	ref := WithBlobTokenName(BlobTokenPrefix+testHandle+"?size=1&type=application%2Fpdf", "invoice.pdf")

	// A destination spelled out in full is a decision, not a default.
	Expect(UploadDestination("reports/2026/summary.pdf", ref, "application/pdf", "upload")).
		To(Equal("reports/2026/summary.pdf"))

	// A directory keeps the file's own name.
	Expect(UploadDestination("reports/2026/", ref, "application/pdf", "upload")).
		To(Equal("reports/2026/invoice.pdf"))

	// Blank means "call it whatever the file is called".
	Expect(UploadDestination("", ref, "application/pdf", "upload")).To(Equal("invoice.pdf"))
	Expect(UploadDestination("  ", ref, "application/pdf", "upload")).To(Equal("invoice.pdf"))

	// With nothing carried, a unique name with the right extension.
	got := UploadDestination("inbox/", "raw", "application/pdf", "report")
	Expect(got).To(HavePrefix("inbox/report-"))
	Expect(got).To(HaveSuffix(".pdf"))
}

// TestFilenameSurvivesDownloadToUploadChain walks the whole path a file takes
// between two actions: a producer writes it under a real name, EmitMediaFile
// turns it into a reference, and an upload asks what to call it.
//
// The chain is what matters here — each link was individually correct before
// and the name still arrived as a random scratch handle, because the blob in
// the middle carried only a handle and a MIME type.
func TestFilenameSurvivesDownloadToUploadChain(t *testing.T) {
	RegisterTestingT(t)
	chdirWorkspace(t)

	f := &Flow{}

	// A producer (Download File, an asset, a Salesforce download) writes the
	// bytes under the name the source gave them.
	produced, err := f.MediaScratchFileNamed("Q3 results.pdf")
	Expect(err).ToNot(HaveOccurred())
	Expect(os.WriteFile(produced, []byte("%PDF-1.7"), 0o600)).To(Succeed())

	ref, err := f.EmitMediaFile(produced)
	Expect(err).ToNot(HaveOccurred())

	// An upload action asks what to call it, having been given no name.
	Expect(UploadFilename("", ref, "application/pdf", "attachment")).To(Equal("Q3 results.pdf"))

	// And a destination that names a directory keeps it too.
	Expect(UploadDestination("archive/2026/", ref, "application/pdf", "attachment")).
		To(Equal("archive/2026/Q3 results.pdf"))

	// The consuming action can also just resolve it and read the base name,
	// which is what the Meta upload does.
	path, _, err := f.ResolveToLocalFile(ref)
	Expect(err).ToNot(HaveOccurred())
	Expect(filepath.Base(path)).To(Equal("Q3 results.pdf"))
}

// TestUploadFilename_NeverProducesAnExtensionlessName is the property the Meta
// failure came down to: whatever arrives, the upload leaves with a name a
// receiving service can act on.
func TestUploadFilename_NeverProducesAnExtensionlessName(t *testing.T) {
	RegisterTestingT(t)

	for _, tc := range []struct{ explicit, ref, mime string }{
		{"", "", "image/jpeg"},
		{"", "raw bytes with no reference", "image/png"},
		{"noextension", "", "application/pdf"},
		{"../../..", "", "video/mp4"},
		{"   ", BlobTokenPrefix + testHandle, "audio/mpeg"},
	} {
		got := UploadFilename(tc.explicit, tc.ref, tc.mime, "upload")
		Expect(got).ToNot(BeEmpty())
		Expect(filepath.Ext(got)).ToNot(BeEmpty(),
			"explicit=%q ref=%q mime=%q produced %q", tc.explicit, tc.ref, tc.mime, got)
		Expect(got).To(Equal(filepath.Base(got)), "must be a single path component")
	}

	// With no MIME to go on there is no extension to invent, but there is
	// still always a name.
	Expect(UploadFilename("", "", "", "upload")).To(HavePrefix("upload-"))
}

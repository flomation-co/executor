package core

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// chdirWorkspace points the process cwd at a fresh temp dir (the workspace the
// resolver confines to). It resolves symlinks first so os.Getwd() matches the
// EvalSymlinks'd paths the resolver computes — on macOS t.TempDir() is under
// /var, a symlink to /private/var, which would otherwise trip containment.
func chdirWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if real, err := filepath.EvalSymlinks(ws); err == nil {
		ws = real
	}
	old, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return ws
}

func TestFileRef_ParseAndIs(t *testing.T) {
	RegisterTestingT(t)
	Expect(IsFileRef("flo:file:a/b.png")).To(BeTrue())
	Expect(IsFileRef("flo:blob:deadbeef")).To(BeFalse())
	Expect(IsFileRef("/etc/passwd")).To(BeFalse())

	rel, ok := ParseFileRef("flo:file:sub/clip.mp4")
	Expect(ok).To(BeTrue())
	Expect(rel).To(Equal(filepath.FromSlash("sub/clip.mp4")))

	_, ok = ParseFileRef("not-a-ref")
	Expect(ok).To(BeFalse())
}

func TestEmitLocalFile_RoundTripsAndRejectsEscape(t *testing.T) {
	RegisterTestingT(t)
	ws := chdirWorkspace(t)
	f := &Flow{}

	// A file inside the workspace emits a relative flo:file: ref.
	abs := filepath.Join(ws, "out", "image.png")
	ref, err := f.EmitLocalFile(abs)
	Expect(err).To(BeNil())
	Expect(ref).To(Equal("flo:file:" + filepath.ToSlash(filepath.Join("out", "image.png"))))

	// A path outside the workspace is refused (no host-path leaks).
	_, err = f.EmitLocalFile("/etc/hosts")
	Expect(err).ToNot(BeNil())
}

func TestResolveToLocalFile_FileRefRoundTrip(t *testing.T) {
	RegisterTestingT(t)
	ws := chdirWorkspace(t)
	f := &Flow{}

	// Create a real file under the workspace and round-trip it.
	Expect(os.MkdirAll(filepath.Join(ws, "media"), 0o700)).To(Succeed())
	src := filepath.Join(ws, "media", "in.bin")
	Expect(os.WriteFile(src, []byte("payload"), 0o600)).To(Succeed())

	ref, err := f.EmitLocalFile(src)
	Expect(err).To(BeNil())

	path, _, err := f.ResolveToLocalFile(ref)
	Expect(err).To(BeNil())
	Expect(path).To(Equal(src))
	b, _ := os.ReadFile(path)
	Expect(string(b)).To(Equal("payload"))
}

func TestResolveToLocalFile_RejectsTraversal(t *testing.T) {
	RegisterTestingT(t)
	chdirWorkspace(t)
	f := &Flow{}

	// A crafted flo:file: value must never escape the workspace.
	for _, evil := range []string{
		"flo:file:../../../../etc/passwd",
		"flo:file:/etc/passwd",
		"flo:file:sub/../../../../etc/hosts",
	} {
		_, _, err := f.ResolveToLocalFile(evil)
		Expect(err).ToNot(BeNil(), "expected %q to be rejected", evil)
	}
}

func TestConfineToWorkspace_NeutralisesEscapes(t *testing.T) {
	RegisterTestingT(t)
	ws := "/ws"
	// Traversal collapses to inside the workspace or is rejected.
	got, err := confineToWorkspace(ws, "a/b/../c.txt")
	Expect(err).To(BeNil())
	Expect(got).To(Equal(filepath.Join(ws, "a", "c.txt")))

	got, err = confineToWorkspace(ws, "../../etc/passwd")
	Expect(err).To(BeNil()) // clamped, not escaped
	Expect(strings.HasPrefix(got, ws)).To(BeTrue())
	Expect(got).To(Equal(filepath.Join(ws, "etc", "passwd")))
}

func TestResolveToLocalFile_Base64ToScratch(t *testing.T) {
	RegisterTestingT(t)
	ws := chdirWorkspace(t)
	f := &Flow{}

	original := []byte("this is a reasonably long payload to look like media bytes")
	value := base64.StdEncoding.EncodeToString(original)

	path, _, err := f.ResolveToLocalFile(value)
	Expect(err).To(BeNil())
	// Written into the workspace media scratch dir.
	Expect(strings.HasPrefix(path, filepath.Join(ws, mediaScratchDir))).To(BeTrue())
	b, _ := os.ReadFile(path)
	Expect(b).To(Equal(original))
}

func TestEmitMediaFile_FallsBackToFileRefWithoutBlobBackend(t *testing.T) {
	RegisterTestingT(t)
	ws := chdirWorkspace(t)
	f := &Flow{}

	// No blob backend on a bare Flow, so Put fails and we fall back to a
	// flo:file: workspace reference (never worse than EmitLocalFile).
	p := filepath.Join(ws, "out.png")
	Expect(os.WriteFile(p, []byte("img"), 0o600)).To(Succeed())

	ref, err := f.EmitMediaFile(p)
	Expect(err).To(BeNil())
	Expect(IsFileRef(ref)).To(BeTrue())
	Expect(IsBlobToken(ref)).To(BeFalse())

	// A missing file errors (unlike the blob path, there's nothing to fall back to).
	_, err = f.EmitMediaFile(filepath.Join(ws, "nope.png"))
	Expect(err).ToNot(BeNil())
}

func TestResolveToBytes_FileRefAndBase64(t *testing.T) {
	RegisterTestingT(t)
	ws := chdirWorkspace(t)
	f := &Flow{}

	// flo:file: → reads the confined workspace file's bytes.
	p := filepath.Join(ws, "d.bin")
	Expect(os.WriteFile(p, []byte("rawbytes"), 0o600)).To(Succeed())
	ref, _ := f.EmitLocalFile(p)
	b, _, err := f.ResolveToBytes(ref)
	Expect(err).To(BeNil())
	Expect(string(b)).To(Equal("rawbytes"))

	// base64 → decoded.
	b2, _, err := f.ResolveToBytes(base64.StdEncoding.EncodeToString([]byte("hello from base64 media")))
	Expect(err).To(BeNil())
	Expect(string(b2)).To(Equal("hello from base64 media"))

	// A crafted flo:file: escape is rejected here too.
	_, _, err = f.ResolveToBytes("flo:file:../../etc/passwd")
	Expect(err).ToNot(BeNil())
}

func TestMediaScratchFile_UniqueAndInScratchDir(t *testing.T) {
	RegisterTestingT(t)
	ws := chdirWorkspace(t)
	f := &Flow{}

	a, err := f.MediaScratchFile("mp3")
	Expect(err).To(BeNil())
	b, err := f.MediaScratchFile(".mp3")
	Expect(err).To(BeNil())
	Expect(a).ToNot(Equal(b))
	Expect(strings.HasPrefix(a, filepath.Join(ws, mediaScratchDir))).To(BeTrue())
	Expect(strings.HasSuffix(a, ".mp3")).To(BeTrue())
	Expect(strings.HasSuffix(b, ".mp3")).To(BeTrue())
}

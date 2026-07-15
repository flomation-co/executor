package resize

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	ic "flomation.app/automate/executor/actions/image"
	. "github.com/onsi/gomega"
)

func TestGeometry(t *testing.T) {
	RegisterTestingT(t)
	Expect(geometry(100, 50)).To(Equal("100x50"))
	Expect(geometry(100, 0)).To(Equal("100"))
	Expect(geometry(0, 50)).To(Equal("x50"))
}

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

func TestResize_MissingImage(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["tool_result"]).To(ContainSubstring("required"))
}

// TestResize_EndToEnd runs the real action against a synthesised image. Skipped
// where ImageMagick isn't installed.
func TestResize_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	magick, err := exec.LookPath("magick")
	if err != nil {
		if magick, err = exec.LookPath("convert"); err != nil {
			t.Skip("ImageMagick not installed; skipping media integration test")
		}
	}

	ws := chdirWorkspace(t)
	flow := &core.Flow{}

	// Synthesise a 200x100 solid-colour PNG.
	src := filepath.Join(ws, "in.png")
	mk := exec.Command(magick, "-size", "200x100", "xc:teal", src)
	if out, err := mk.CombinedOutput(); err != nil {
		t.Fatalf("synthesise image: %v\n%s", err, out)
	}

	ref, err := flow.EmitLocalFile(src)
	Expect(err).To(BeNil())

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "image", Type: core.ConnectionTypeString, Value: ref},
		{Name: "width", Type: core.ConnectionTypeInteger, Value: 100},
		{Name: "fit", Type: core.ConnectionTypeString, Value: "fit"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	// 200x100 fit to width 100 → 100x50 (aspect preserved).
	Expect(out["width"]).To(Equal(100))
	Expect(out["height"]).To(Equal(50))

	imageRef, _ := out["image"].(string)
	Expect(core.IsFileRef(imageRef)).To(BeTrue())

	// The emitted reference resolves to a real image whose dimensions Identify agrees on.
	path, _, err := flow.ResolveToLocalFile(imageRef)
	Expect(err).To(BeNil())
	info, err := ic.Identify(flow.GoContext(), path)
	Expect(err).To(BeNil())
	Expect(info.Width).To(Equal(100))
	Expect(info.Height).To(Equal(50))
}

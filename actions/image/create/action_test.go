package create

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	ic "flomation.app/automate/executor/actions/image"
	. "github.com/onsi/gomega"
)

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

func TestCreate_BadSize(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "width", Type: core.ConnectionTypeInteger, Value: 0},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
}

// TestCreate_EndToEnd is a generative "source" action — no media input.
func TestCreate_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	if _, err := exec.LookPath("magick"); err != nil {
		if _, err := exec.LookPath("convert"); err != nil {
			t.Skip("ImageMagick not installed")
		}
	}
	chdirWorkspace(t)
	flow := &core.Flow{}

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "width", Type: core.ConnectionTypeInteger, Value: 320},
		{Name: "height", Type: core.ConnectionTypeInteger, Value: 240},
		{Name: "colour", Type: core.ConnectionTypeString, Value: "teal"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["width"]).To(Equal(320))

	ref, _ := out["image"].(string)
	Expect(core.IsFileRef(ref)).To(BeTrue())
	path, _, err := flow.ResolveToLocalFile(ref)
	Expect(err).To(BeNil())
	info, err := ic.Identify(flow.GoContext(), path)
	Expect(err).To(BeNil())
	Expect(info.Width).To(Equal(320))
	Expect(info.Height).To(Equal(240))
}

package psd_rasterise

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	ic "flomation.app/automate/executor/actions/image"
	. "github.com/onsi/gomega"
)

func buildPSD(t *testing.T, path string) {
	t.Helper()
	args := []string{
		"-size", "400x300", "xc:navy", "-set", "label", "background",
		"(", "-size", "200x50", "xc:red", "-set", "label", "title_text", "-repage", "+100+60", ")",
		path,
	}
	out, err := exec.Command("magick", args...).CombinedOutput() // #nosec G204 -- test fixture, fixed argv
	Expect(err).To(BeNil(), "magick build: %s", string(out))
}

func TestPSDRasterise(t *testing.T) {
	RegisterTestingT(t)
	if _, e := exec.LookPath("magick"); e != nil {
		t.Skip("imagemagick not installed")
	}
	d := t.TempDir()
	if r, e := filepath.EvalSymlinks(d); e == nil {
		d = r
	}
	o, _ := os.Getwd()
	_ = os.Chdir(d)
	t.Cleanup(func() { _ = os.Chdir(o) })

	flow := &core.Flow{}
	psdPath, err := flow.MediaScratchFile("psd")
	Expect(err).To(BeNil())
	buildPSD(t, psdPath)
	ref, err := flow.EmitLocalFile(psdPath)
	Expect(err).To(BeNil())

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "psd", Type: core.ConnectionTypeString, Value: ref},
		{Name: "format", Type: core.ConnectionTypeString, Value: "png"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["width"]).To(Equal(400))
	Expect(out["height"]).To(Equal(300))

	// The emitted reference must resolve to a real, valid PNG of canvas size.
	p, _, err := flow.ResolveToLocalFile(out["image"].(string))
	Expect(err).To(BeNil())
	info, err := ic.Identify(flow.GoContext(), p)
	Expect(err).To(BeNil())
	Expect(info.Width).To(Equal(400))
	Expect(info.Height).To(Equal(300))
}

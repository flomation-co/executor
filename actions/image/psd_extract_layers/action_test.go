package psd_extract_layers

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func buildPSD(t *testing.T, path string) {
	t.Helper()
	args := []string{
		"-size", "400x300", "xc:navy", "-set", "label", "background",
		"(", "-size", "200x50", "xc:red", "-set", "label", "title_text", "-repage", "+100+60", ")",
		"(", "-size", "120x30", "xc:yellow", "-set", "label", "subtitle_text", "-repage", "+140+200", ")",
		path,
	}
	out, err := exec.Command("magick", args...).CombinedOutput() // #nosec G204 -- test fixture, fixed argv
	Expect(err).To(BeNil(), "magick build: %s", string(out))
}

func TestExtractNamedLayers(t *testing.T) {
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

	// Filter to a single named layer.
	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "psd", Type: core.ConnectionTypeString, Value: ref},
		{Name: "names", Type: core.ConnectionTypeString, Value: "title_text"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))

	var got []extracted
	Expect(json.Unmarshal([]byte(out["layers"].(string)), &got)).To(Succeed())
	Expect(got).To(HaveLen(1))
	Expect(got[0].Name).To(Equal("title_text"))
	Expect(got[0].Width).To(Equal(200))
	Expect(got[0].Ref).To(HavePrefix("flo:"))

	// The extracted layer must resolve to a real image.
	p, _, err := flow.ResolveToLocalFile(got[0].Ref)
	Expect(err).To(BeNil())
	fi, e := os.Stat(p)
	Expect(e).To(BeNil())
	Expect(fi.Size()).To(BeNumerically(">", 0))
}

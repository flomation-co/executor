package psd_info

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// buildPSD writes a 400x300 PSD with two named layers (title_text at +100+60,
// subtitle_text at +140+200) using solid fills — no fonts needed, so the test
// is hermetic and independent of the host's font configuration.
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

func TestPSDInfo(t *testing.T) {
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
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	// Canvas comes from the composite (scene 0), not the last layer.
	Expect(out["width"]).To(Equal(400))
	Expect(out["height"]).To(Equal(300))
	Expect(out["colour_mode"]).To(ContainSubstring("RGB"))

	var ls []Layer
	Expect(json.Unmarshal([]byte(out["layers"].(string)), &ls)).To(Succeed())
	var title *Layer
	for i := range ls {
		if ls[i].Name == "title_text" {
			title = &ls[i]
		}
	}
	Expect(title).ToNot(BeNil(), "title_text layer should be found by name")
	Expect(title.X).To(Equal(100))
	Expect(title.Y).To(Equal(60))
	Expect(title.Width).To(Equal(200))
	Expect(title.Height).To(Equal(50))
}

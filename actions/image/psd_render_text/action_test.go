package psd_render_text

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fogleman/gg"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// buildPSD writes a PSD with a real full-canvas background LAYER (navy) plus a
// red "headline" layer at +100+60 — the shape a personalisation template has.
// scene 0 is the flattened composite; scene 1 the background; scene 2 the target.
func buildPSD(t *testing.T, path string) {
	t.Helper()
	args := []string{
		"-size", "400x300", "xc:navy",
		"(", "-size", "400x300", "xc:navy", "-set", "label", "background", ")",
		"(", "-size", "200x50", "xc:red", "-set", "label", "headline", "-repage", "+100+60", ")",
		path,
	}
	out, err := exec.Command("magick", args...).CombinedOutput() // #nosec G204 -- test fixture, fixed argv
	Expect(err).To(BeNil(), "magick build: %s", string(out))
}

func setup(t *testing.T) (*core.Flow, string) {
	t.Helper()
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
	return flow, ref
}

func TestRenderText_RemovesOldLayerAndRenders(t *testing.T) {
	RegisterTestingT(t)
	flow, ref := setup(t)

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "psd", Type: core.ConnectionTypeString, Value: ref},
		{Name: "layer_name", Type: core.ConnectionTypeString, Value: "headline"},
		{Name: "text", Type: core.ConnectionTypeText, Value: "Hi"},
		{Name: "font_size", Type: core.ConnectionTypeInteger, Value: 20},
		{Name: "colour", Type: core.ConnectionTypeString, Value: "#ffffff"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true), "error: %v", out["error"])
	Expect(out["width"]).To(Equal(400))
	Expect(out["height"]).To(Equal(300))

	p, _, err := flow.ResolveToLocalFile(out["image"].(string))
	Expect(err).To(BeNil())
	img, err := gg.LoadPNG(p)
	Expect(err).To(BeNil())
	Expect(img.Bounds().Dx()).To(Equal(400))

	// A corner of the old headline box (near +105,+65) is not covered by the new
	// centred text — it must now be the navy BACKGROUND, not the old red layer.
	r, g, b, _ := img.At(105, 65).RGBA()
	Expect(r >> 8).To(BeNumerically("<", 60), "old red layer should be gone (low red)")
	Expect(b >> 8).To(BeNumerically(">", 90), "navy background should show through (high blue)")
	_ = g
}

func TestRenderText_UnknownLayer(t *testing.T) {
	RegisterTestingT(t)
	flow, ref := setup(t)

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "psd", Type: core.ConnectionTypeString, Value: ref},
		{Name: "layer_name", Type: core.ConnectionTypeString, Value: "does_not_exist"},
		{Name: "text", Type: core.ConnectionTypeText, Value: "Hi"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("headline"), "error should list available layer names")
}

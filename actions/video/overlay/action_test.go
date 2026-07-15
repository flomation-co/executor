package overlay

import (
	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOverlayXY(t *testing.T) {
	RegisterTestingT(t)
	Expect(overlayXY("Top", 20)).To(Equal("(main_w-overlay_w)/2:20"))
	Expect(overlayXY("Bottom", 20)).To(Equal("(main_w-overlay_w)/2:main_h-overlay_h-20"))
	Expect(overlayXY("Center", 0)).To(Equal("(main_w-overlay_w)/2:(main_h-overlay_h)/2"))
}
func TestOverlay_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	magick, e1 := exec.LookPath("magick")
	_, e2 := exec.LookPath("ffmpeg")
	if e1 != nil || e2 != nil {
		t.Skip("need magick+ffmpeg")
	}
	d := t.TempDir()
	if r, e := filepath.EvalSymlinks(d); e == nil {
		d = r
	}
	o, _ := os.Getwd()
	_ = os.Chdir(d)
	t.Cleanup(func() { _ = os.Chdir(o) })
	flow := &core.Flow{}
	base := filepath.Join(d, "b.mp4")
	if o, e := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "testsrc=duration=2:size=320x240:rate=15", "-pix_fmt", "yuv420p", base).CombinedOutput(); e != nil {
		t.Skipf("synth: %v\n%s", e, o)
	}
	logo := filepath.Join(d, "logo.png")
	if o, e := exec.Command(magick, "-size", "60x60", "xc:yellow", logo).CombinedOutput(); e != nil {
		t.Skipf("synth: %v\n%s", e, o)
	}
	bref, _ := flow.EmitLocalFile(base)
	lref, _ := flow.EmitLocalFile(logo)
	out, err := Execute(flow, nil, []*core.Connection{{Name: "video", Type: core.ConnectionTypeString, Value: bref}, {Name: "overlay", Type: core.ConnectionTypeString, Value: lref}, {Name: "position", Type: core.ConnectionTypeString, Value: "Bottom"}})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	p, _, err := flow.ResolveToLocalFile(out["video"].(string))
	Expect(err).To(BeNil())
	fi, _ := os.Stat(p)
	Expect(fi.Size()).To(BeNumerically(">", 0))
}

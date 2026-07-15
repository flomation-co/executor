package animated_title

import (
	"image"
	_ "image/png" // register the PNG decoder for image.Decode
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
	. "github.com/onsi/gomega"
)

func TestTitleAnim(t *testing.T) {
	RegisterTestingT(t)
	// At t=0 the intro hasn't progressed: fully offset, alpha 0-ish.
	dx, _, a0 := titleAnim("slide_left", 0, 4, 1280, 300)
	Expect(dx).To(BeNumerically("~", 1280, 1))
	Expect(a0).To(BeNumerically("<", 0.05))
	// Past the intro: centred, opaque.
	dx2, _, a2 := titleAnim("slide_left", 1.0, 4, 1280, 300)
	Expect(dx2).To(BeNumerically("~", 0, 1))
	Expect(a2).To(Equal(1.0))
	// Fade has no offset.
	fdx, fdy, _ := titleAnim("fade", 1.0, 4, 1280, 300)
	Expect(fdx).To(Equal(0.0))
	Expect(fdy).To(Equal(0.0))
}

func TestAnimatedTitle_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	if _, e := exec.LookPath("ffmpeg"); e != nil {
		t.Skip("ffmpeg not installed")
	}
	d := t.TempDir()
	if r, e := filepath.EvalSymlinks(d); e == nil {
		d = r
	}
	o, _ := os.Getwd()
	_ = os.Chdir(d)
	t.Cleanup(func() { _ = os.Chdir(o) })
	flow := &core.Flow{}
	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "text", Type: core.ConnectionTypeText, Value: "Hello Flomation"},
		{Name: "duration_seconds", Type: core.ConnectionTypeString, Value: "1"},
		{Name: "fps", Type: core.ConnectionTypeInteger, Value: 10},
		{Name: "width", Type: core.ConnectionTypeInteger, Value: 640},
		{Name: "height", Type: core.ConnectionTypeInteger, Value: 160},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))

	p, _, err := flow.ResolveToLocalFile(out["video"].(string))
	Expect(err).To(BeNil())

	// It must be a valid, correctly-sized image stream.
	pr, err := vc.Probe(flow.GoContext(), p)
	Expect(err).To(BeNil())
	Expect(pr.Width).To(Equal(640))
	Expect(pr.Height).To(Equal(160))

	// Regression: the graphic MUST carry a real (non-flat) alpha channel, or it is
	// useless as an overlay. A previous encode (qtrle .mov, then libvpx WebM) either
	// wasn't web-playable or silently dropped alpha; APNG must preserve it. If the
	// output had no alpha, ffmpeg's alphaextract writes NOTHING, so the decode fails.
	assertHasAlpha(t, p)
}

// assertHasAlpha extracts the alpha plane of every frame of the rendered graphic and
// fails unless, across the whole animation, it contains both transparent and opaque
// pixels (i.e. a genuine, non-flat alpha channel). It must scan ALL frames because the
// intro frame is fully transparent (the title fades in from alpha 0), so a single-frame
// probe would see a blank plane and give a false negative.
func assertHasAlpha(t *testing.T, path string) {
	t.Helper()
	pat := filepath.Join(filepath.Dir(path), "alpha_%03d.png")
	// #nosec G204 -- fixed argv, path is a workspace-confined temp file.
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-i", path,
		"-vf", "alphaextract,format=gray", pat)
	outp, runErr := cmd.CombinedOutput()
	Expect(runErr).To(BeNil(), "alphaextract failed (no alpha channel?): %s", string(outp))

	frames, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "alpha_*.png"))
	Expect(frames).ToNot(BeEmpty(), "alphaextract produced no frames — transparency was lost")

	var min, max uint32 = 65535, 0
	for _, fp := range frames {
		f, err := os.Open(fp) // #nosec G304 -- test-controlled path
		Expect(err).To(BeNil())
		img, _, derr := image.Decode(f)
		_ = f.Close()
		Expect(derr).To(BeNil())
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				l, _, _, _ := img.At(x, y).RGBA()
				if l < min {
					min = l
				}
				if l > max {
					max = l
				}
			}
		}
	}
	Expect(max).To(BeNumerically(">", min), "alpha plane is flat — transparency was lost")
}

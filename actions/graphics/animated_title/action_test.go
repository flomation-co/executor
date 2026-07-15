package animated_title

import (
	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
	. "github.com/onsi/gomega"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
	pr, err := vc.Probe(flow.GoContext(), p)
	Expect(err).To(BeNil())
	Expect(pr.DurationSeconds).To(BeNumerically("~", 1.0, 0.3))
	Expect(pr.Width).To(Equal(640))
}

package ken_burns

import (
	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
	. "github.com/onsi/gomega"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func wsdir(t *testing.T) string {
	d := t.TempDir()
	if r, e := filepath.EvalSymlinks(d); e == nil {
		d = r
	}
	o, _ := os.Getwd()
	_ = os.Chdir(d)
	t.Cleanup(func() { _ = os.Chdir(o) })
	return d
}
func TestKenBurns_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	magick, e1 := exec.LookPath("magick")
	_, e2 := exec.LookPath("ffmpeg")
	if e1 != nil || e2 != nil {
		t.Skip("need magick+ffmpeg")
	}
	d := wsdir(t)
	flow := &core.Flow{}
	src := filepath.Join(d, "in.png")
	if o, e := exec.Command(magick, "-size", "400x300", "gradient:red-blue", src).CombinedOutput(); e != nil {
		t.Skipf("synth: %v\n%s", e, o)
	}
	ref, _ := flow.EmitLocalFile(src)
	out, err := Execute(flow, nil, []*core.Connection{{Name: "image", Type: core.ConnectionTypeString, Value: ref}, {Name: "duration_seconds", Type: core.ConnectionTypeString, Value: "2"}, {Name: "width", Type: core.ConnectionTypeInteger, Value: 320}})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	p, _, err := flow.ResolveToLocalFile(out["video"].(string))
	Expect(err).To(BeNil())
	pr, err := vc.Probe(flow.GoContext(), p)
	Expect(err).To(BeNil())
	Expect(pr.DurationSeconds).To(BeNumerically(">", 1.5))
}

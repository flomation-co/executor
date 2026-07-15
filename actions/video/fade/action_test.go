package fade

import (
	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFade_EndToEnd(t *testing.T) {
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
	v := filepath.Join(d, "in.mp4")
	if o, e := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "testsrc=duration=2:size=160x120:rate=15", "-pix_fmt", "yuv420p", v).CombinedOutput(); e != nil {
		t.Skipf("synth: %v\n%s", e, o)
	}
	ref, _ := flow.EmitLocalFile(v)
	out, err := Execute(flow, nil, []*core.Connection{{Name: "video", Type: core.ConnectionTypeString, Value: ref}, {Name: "fade_in_seconds", Type: core.ConnectionTypeString, Value: "0.5"}, {Name: "fade_out_seconds", Type: core.ConnectionTypeString, Value: "0.5"}})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
}

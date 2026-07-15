package speed

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
	. "github.com/onsi/gomega"
)

func TestSpeed_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	ws := t.TempDir()
	if r, e := filepath.EvalSymlinks(ws); e == nil {
		ws = r
	}
	old, _ := os.Getwd()
	_ = os.Chdir(ws)
	t.Cleanup(func() { _ = os.Chdir(old) })

	flow := &core.Flow{}
	vid := filepath.Join(ws, "in.mp4")
	// 2s video WITH an audio track, so the atempo path is exercised.
	if out, e := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=2:size=128x96:rate=15",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-pix_fmt", "yuv420p", "-shortest", vid).CombinedOutput(); e != nil {
		t.Skipf("synthesise video (ffmpeg setup issue): %v\n%s", e, out)
	}
	ref, _ := flow.EmitLocalFile(vid)

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "video", Type: core.ConnectionTypeString, Value: ref},
		{Name: "factor", Type: core.ConnectionTypeString, Value: "2"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))

	p, _, err := flow.ResolveToLocalFile(out["video"].(string))
	Expect(err).To(BeNil())
	// 2s at 2x → ~1s.
	probe, err := vc.Probe(flow.GoContext(), p)
	Expect(err).To(BeNil())
	Expect(probe.DurationSeconds).To(BeNumerically("<", 1.6))
}

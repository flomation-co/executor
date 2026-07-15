package gif

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestGif_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	ws := t.TempDir()
	if real, err := filepath.EvalSymlinks(ws); err == nil {
		ws = real
	}
	old, _ := os.Getwd()
	_ = os.Chdir(ws)
	t.Cleanup(func() { _ = os.Chdir(old) })

	flow := &core.Flow{}
	vid := filepath.Join(ws, "in.mp4")
	if out, err := exec.Command("ffmpeg", "-y", "-f", "lavfi",
		"-i", "testsrc=duration=2:size=160x120:rate=15", "-pix_fmt", "yuv420p", vid).CombinedOutput(); err != nil {
		t.Skipf("synthesise video (ffmpeg setup issue): %v\n%s", err, out)
	}
	ref, _ := flow.EmitLocalFile(vid)

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "video", Type: core.ConnectionTypeString, Value: ref},
		{Name: "duration_seconds", Type: core.ConnectionTypeString, Value: "1"},
		{Name: "fps", Type: core.ConnectionTypeString, Value: "8"},
		{Name: "width", Type: core.ConnectionTypeInteger, Value: 120},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))

	path, _, err := flow.ResolveToLocalFile(out["gif"].(string))
	Expect(err).To(BeNil())
	// Confirm it's actually a GIF (magic bytes GIF8).
	f, _ := os.Open(path)
	defer func() { _ = f.Close() }()
	head := make([]byte, 4)
	_, _ = f.Read(head)
	Expect(string(head)).To(Equal("GIF8"))
}

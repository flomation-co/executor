package waveform

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func chdirWS(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if r, e := filepath.EvalSymlinks(ws); e == nil {
		ws = r
	}
	old, _ := os.Getwd()
	_ = os.Chdir(ws)
	t.Cleanup(func() { _ = os.Chdir(old) })
	return ws
}

func TestWaveform_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	ws := chdirWS(t)
	flow := &core.Flow{}
	aud := filepath.Join(ws, "a.mp3")
	if out, e := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=2", aud).CombinedOutput(); e != nil {
		t.Skipf("synthesise audio (ffmpeg setup issue): %v\n%s", e, out)
	}
	ref, _ := flow.EmitLocalFile(aud)

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "audio", Type: core.ConnectionTypeString, Value: ref},
		{Name: "width", Type: core.ConnectionTypeInteger, Value: 400},
		{Name: "height", Type: core.ConnectionTypeInteger, Value: 100},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	p, _, err := flow.ResolveToLocalFile(out["image"].(string))
	Expect(err).To(BeNil())
	f, _ := os.Open(p)
	defer func() { _ = f.Close() }()
	head := make([]byte, 4)
	_, _ = f.Read(head)
	Expect(head).To(Equal([]byte{0x89, 'P', 'N', 'G'})) // PNG magic
}

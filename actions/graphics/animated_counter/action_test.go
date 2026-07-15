package animated_counter

import (
	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAnimatedCounter_EndToEnd(t *testing.T) {
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
		{Name: "from", Type: core.ConnectionTypeString, Value: "0"},
		{Name: "to", Type: core.ConnectionTypeString, Value: "250"},
		{Name: "prefix", Type: core.ConnectionTypeString, Value: "£"},
		{Name: "duration_seconds", Type: core.ConnectionTypeString, Value: "1"},
		{Name: "fps", Type: core.ConnectionTypeInteger, Value: 10},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	p, _, err := flow.ResolveToLocalFile(out["video"].(string))
	Expect(err).To(BeNil())
	fi, _ := os.Stat(p)
	Expect(fi.Size()).To(BeNumerically(">", 0))
}

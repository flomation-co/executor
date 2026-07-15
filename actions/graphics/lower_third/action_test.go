package lower_third

import (
	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSlideOffset(t *testing.T) {
	RegisterTestingT(t)
	Expect(slideOffset(0, 5, 1280)).To(BeNumerically("~", -1280, 1)) // fully off-screen at start
	Expect(slideOffset(2.5, 5, 1280)).To(Equal(0.0))                 // held on-screen mid-way
}
func TestLowerThird_EndToEnd(t *testing.T) {
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
		{Name: "title", Type: core.ConnectionTypeString, Value: "Jane Doe"},
		{Name: "subtitle", Type: core.ConnectionTypeString, Value: "Head of Product"},
		{Name: "duration_seconds", Type: core.ConnectionTypeString, Value: "1"},
		{Name: "fps", Type: core.ConnectionTypeInteger, Value: 10},
		{Name: "width", Type: core.ConnectionTypeInteger, Value: 640},
		{Name: "height", Type: core.ConnectionTypeInteger, Value: 120},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	p, _, err := flow.ResolveToLocalFile(out["video"].(string))
	Expect(err).To(BeNil())
	fi, _ := os.Stat(p)
	Expect(fi.Size()).To(BeNumerically(">", 0))
}

package montage

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestMontage_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	magick, err := exec.LookPath("magick")
	if err != nil {
		t.Skip("ImageMagick not installed")
	}
	d := t.TempDir()
	if r, e := filepath.EvalSymlinks(d); e == nil {
		d = r
	}
	old, _ := os.Getwd()
	_ = os.Chdir(d)
	t.Cleanup(func() { _ = os.Chdir(old) })

	flow := &core.Flow{}
	mk := func(name, col string) string {
		p := filepath.Join(d, name)
		if o, e := exec.Command(magick, "-size", "80x80", "xc:"+col, p).CombinedOutput(); e != nil {
			t.Skipf("synth: %v\n%s", e, o)
		}
		r, _ := flow.EmitLocalFile(p)
		return r
	}
	a, b := mk("a.png", "red"), mk("b.png", "blue")
	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "images", Type: core.ConnectionTypeText, Value: a + "\n" + b},
		{Name: "columns", Type: core.ConnectionTypeInteger, Value: 2},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true)) // proves montage runs with the -limit prefix
	Expect(out["image_count"]).To(Equal(2))
}

package palette

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestPalette_EndToEnd(t *testing.T) {
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
	src := filepath.Join(d, "grad.png")
	// A gradient gives several distinct colours to quantise.
	if o, e := exec.Command(magick, "-size", "100x100", "gradient:red-blue", src).CombinedOutput(); e != nil {
		t.Skipf("synth: %v\n%s", e, o)
	}
	ref, _ := flow.EmitLocalFile(src)
	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "image", Type: core.ConnectionTypeString, Value: ref},
		{Name: "count", Type: core.ConnectionTypeInteger, Value: 4},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	cols, _ := out["colours"].([]string)
	Expect(len(cols)).To(BeNumerically(">", 0))
	Expect(cols[0]).To(MatchRegexp(`^#[0-9A-F]{6}$`))
}

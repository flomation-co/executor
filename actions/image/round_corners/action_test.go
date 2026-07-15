package round_corners

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func ws(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	if r, e := filepath.EvalSymlinks(d); e == nil {
		d = r
	}
	old, _ := os.Getwd()
	_ = os.Chdir(d)
	t.Cleanup(func() { _ = os.Chdir(old) })
	return d
}

func TestRoundCorners_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	magick, err := exec.LookPath("magick")
	if err != nil {
		t.Skip("ImageMagick not installed")
	}
	d := ws(t)
	flow := &core.Flow{}
	src := filepath.Join(d, "in.png")
	if o, e := exec.Command(magick, "-size", "100x100", "xc:red", src).CombinedOutput(); e != nil {
		t.Skipf("synth: %v\n%s", e, o)
	}
	ref, _ := flow.EmitLocalFile(src)
	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "image", Type: core.ConnectionTypeString, Value: ref},
		{Name: "radius", Type: core.ConnectionTypeInteger, Value: 20},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	p, _, err := flow.ResolveToLocalFile(out["image"].(string))
	Expect(err).To(BeNil())
	f, _ := os.Open(p)
	defer func() { _ = f.Close() }()
	head := make([]byte, 4)
	_, _ = f.Read(head)
	Expect(head).To(Equal([]byte{0x89, 'P', 'N', 'G'}))
}

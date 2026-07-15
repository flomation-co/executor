package to_pdf

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestToPDF_RequiresImages(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
}

func TestToPDF_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	magick, err := exec.LookPath("magick")
	if err != nil {
		t.Skip("ImageMagick not installed")
	}
	ws := t.TempDir()
	if real, e := filepath.EvalSymlinks(ws); e == nil {
		ws = real
	}
	old, _ := os.Getwd()
	_ = os.Chdir(ws)
	t.Cleanup(func() { _ = os.Chdir(old) })

	flow := &core.Flow{}
	mk := func(name, colour string) string {
		p := filepath.Join(ws, name)
		if out, e := exec.Command(magick, "-size", "100x100", "xc:"+colour, p).CombinedOutput(); e != nil {
			t.Skipf("synthesise image: %v\n%s", e, out)
		}
		r, _ := flow.EmitLocalFile(p)
		return r
	}
	a := mk("a.png", "red")
	b := mk("b.png", "blue")

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "images", Type: core.ConnectionTypeText, Value: a + "\n" + b},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["page_count"]).To(Equal(2))

	path, _, err := flow.ResolveToLocalFile(out["pdf"].(string))
	Expect(err).To(BeNil())
	f, _ := os.Open(path)
	defer func() { _ = f.Close() }()
	head := make([]byte, 5)
	_, _ = f.Read(head)
	Expect(string(head)).To(Equal("%PDF-")) // real PDF magic bytes
}

package adjust

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestEffectArgs(t *testing.T) {
	RegisterTestingT(t)
	Expect(effectArgs("grayscale", 0)).To(Equal([]string{"-colorspace", "Gray"}))
	Expect(effectArgs("blur", 3)).To(Equal([]string{"-blur", "0x3"}))
	Expect(effectArgs("sharpen", 2)).To(Equal([]string{"-sharpen", "0x2"}))
	Expect(effectArgs("brightness", 10)).To(Equal([]string{"-brightness-contrast", "10x0"}))
	Expect(effectArgs("contrast", 15)).To(Equal([]string{"-brightness-contrast", "0x15"}))
	Expect(effectArgs("sepia", 0)).To(Equal([]string{"-sepia-tone", "80%"})) // zero → sensible default
}

func TestClamp(t *testing.T) {
	RegisterTestingT(t)
	Expect(clamp(0, 0, 100, 80)).To(Equal(80)) // unset → default
	Expect(clamp(150, 0, 100, 5)).To(Equal(100))
	Expect(clamp(-10, -100, 100, 20)).To(Equal(-10))
}

func TestAdjust_EndToEnd(t *testing.T) {
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
	src := filepath.Join(ws, "in.png")
	if out, e := exec.Command(magick, "-size", "64x64", "xc:red", src).CombinedOutput(); e != nil {
		t.Skipf("synthesise image: %v\n%s", e, out)
	}
	ref, _ := flow.EmitLocalFile(src)

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "image", Type: core.ConnectionTypeString, Value: ref},
		{Name: "effect", Type: core.ConnectionTypeString, Value: "grayscale"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	path, _, err := flow.ResolveToLocalFile(out["image"].(string))
	Expect(err).To(BeNil())
	fi, _ := os.Stat(path)
	Expect(fi.Size()).To(BeNumerically(">", 0))
}

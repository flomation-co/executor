package burn_text

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestYExpr(t *testing.T) {
	RegisterTestingT(t)
	Expect(yExpr("top")).To(Equal("40"))
	Expect(yExpr("center")).To(Equal("(h-text_h)/2"))
	Expect(yExpr("bottom")).To(Equal("h-text_h-40"))
}

func TestBurnText_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, ok := findFont(""); !ok {
		t.Skip("no system font available for drawtext")
	}
	// Homebrew ffmpeg is often built without libfreetype (no drawtext filter);
	// standard distro ffmpeg on the runner hosts includes it.
	filters, _ := exec.Command("ffmpeg", "-hide_banner", "-filters").CombinedOutput()
	if !strings.Contains(string(filters), "drawtext") {
		t.Skip("this ffmpeg build lacks the drawtext filter (no libfreetype)")
	}

	d := t.TempDir()
	if r, e := filepath.EvalSymlinks(d); e == nil {
		d = r
	}
	old, _ := os.Getwd()
	_ = os.Chdir(d)
	t.Cleanup(func() { _ = os.Chdir(old) })

	flow := &core.Flow{}
	vid := filepath.Join(d, "in.mp4")
	if o, e := exec.Command("ffmpeg", "-y", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=15", "-pix_fmt", "yuv420p", vid).CombinedOutput(); e != nil {
		t.Skipf("synth (ffmpeg setup): %v\n%s", e, o)
	}
	ref, _ := flow.EmitLocalFile(vid)

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "video", Type: core.ConnectionTypeString, Value: ref},
		{Name: "text", Type: core.ConnectionTypeText, Value: "Hello: it's a test"}, // colon + quote survive via textfile
		{Name: "position", Type: core.ConnectionTypeString, Value: "bottom"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	p, _, err := flow.ResolveToLocalFile(out["video"].(string))
	Expect(err).To(BeNil())
	fi, err := os.Stat(p)
	Expect(err).To(BeNil())
	Expect(fi.Size()).To(BeNumerically(">", 0))
}

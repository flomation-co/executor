package concat

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestListSplit_HandlesNewlinesAndCommas(t *testing.T) {
	RegisterTestingT(t)
	parts := listSplit.Split("a\nb, c\r\n\nd", -1)
	var got []string
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			got = append(got, s)
		}
	}
	Expect(got).To(Equal([]string{"a", "b", "c", "d"}))
}

func TestConcat_NeedsTwo(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "videos", Type: core.ConnectionTypeText, Value: "flo:file:only-one.mp4"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["tool_result"]).To(ContainSubstring("at least two"))
}

func chdirWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if real, err := filepath.EvalSymlinks(ws); err == nil {
		ws = real
	}
	old, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return ws
}

func TestConcat_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	ws := chdirWorkspace(t)
	flow := &core.Flow{}

	// Two identically-encoded 1s clips (so stream-copy concat works).
	mk := func(name string) string {
		p := filepath.Join(ws, name)
		c := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "testsrc=duration=1:size=128x128:rate=15",
			"-pix_fmt", "yuv420p", "-c:v", "libx264", p)
		if out, err := c.CombinedOutput(); err != nil {
			t.Skipf("could not synthesise clip (ffmpeg setup issue): %v\n%s", err, out)
		}
		return p
	}
	a, _ := flow.EmitLocalFile(mk("a.mp4"))
	b, _ := flow.EmitLocalFile(mk("b.mp4"))

	out, err := Execute(flow, nil, []*core.Connection{
		{Name: "videos", Type: core.ConnectionTypeText, Value: a + "\n" + b},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))

	path, _, err := flow.ResolveToLocalFile(out["video"].(string))
	Expect(err).To(BeNil())
	fi, err := os.Stat(path)
	Expect(err).To(BeNil())
	Expect(fi.Size()).To(BeNumerically(">", 0))
}

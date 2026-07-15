package extract_audio

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestAudioCodec(t *testing.T) {
	RegisterTestingT(t)
	Expect(audioCodec("mp3")).To(Equal("libmp3lame"))
	Expect(audioCodec("aac")).To(Equal("aac"))
	Expect(audioCodec("wav")).To(Equal("pcm_s16le"))
	Expect(audioCodec("ogg")).To(Equal("libvorbis"))
	Expect(audioCodec("anything-else")).To(Equal("libmp3lame")) // safe default
}

// chdirWorkspace points cwd at a fresh temp dir (resolving symlinks so the
// workspace-confinement checks in core.ResolveToLocalFile line up on macOS).
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

// TestExtractAudio_EndToEnd runs the real action against a synthesised video.
// Skipped where ffmpeg isn't installed (e.g. CI before runner provisioning).
func TestExtractAudio_EndToEnd(t *testing.T) {
	RegisterTestingT(t)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed; skipping media integration test")
	}

	ws := chdirWorkspace(t)
	flow := &core.Flow{}

	// Synthesise a 1s video with an audio track.
	vid := filepath.Join(ws, "in.mp4")
	synth := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=128x128:rate=10",
		"-shortest", "-pix_fmt", "yuv420p", vid)
	if out, err := synth.CombinedOutput(); err != nil {
		// ffmpeg is present but couldn't even build the fixture (e.g. a broken
		// dylib link on a dev box). That's an environment problem, not a code
		// failure — skip rather than fail.
		t.Skipf("could not synthesise test video (ffmpeg setup issue): %v\n%s", err, out)
	}

	ref, err := flow.EmitLocalFile(vid)
	Expect(err).To(BeNil())

	inputs := []*core.Connection{
		{Name: "video", Type: core.ConnectionTypeString, Value: ref},
		{Name: "format", Type: core.ConnectionTypeString, Value: "mp3"},
		{Name: "bitrate", Type: core.ConnectionTypeString, Value: "128k"},
	}

	out, err := Execute(flow, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["format"]).To(Equal("mp3"))

	audioRef, _ := out["audio"].(string)
	Expect(core.IsFileRef(audioRef)).To(BeTrue())

	// The emitted reference resolves to a real, non-empty audio file.
	audioPath, _, err := flow.ResolveToLocalFile(audioRef)
	Expect(err).To(BeNil())
	fi, err := os.Stat(audioPath)
	Expect(err).To(BeNil())
	Expect(fi.Size()).To(BeNumerically(">", 0))
}

// TestExtractAudio_MissingVideo checks the typed-error path without ffmpeg.
func TestExtractAudio_MissingVideo(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["tool_result"]).To(ContainSubstring("required"))
}

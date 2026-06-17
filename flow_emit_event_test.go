package core

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what was
// written. Reading happens on a goroutine so a large write can never block.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()

	_ = w.Close()
	os.Stdout = old
	return <-done
}

// TestEmitNodeEventTruncatesLargeOutputs is the regression guard for the
// stdout pipe deadlock: a large output value (e.g. base64 audio from
// elevenlabs/text_to_speech) must NOT be streamed verbatim in a __NODE__
// event, because an over-long line overflows the runner's stdout scanner
// buffer, stalls the pipe, and hangs the executor on the write.
func TestEmitNodeEventTruncatesLargeOutputs(t *testing.T) {
	RegisterTestingT(t)

	// ~10MB string, far larger than the runner's 1MB scanner buffer.
	huge := strings.Repeat("A", 10<<20)
	outputs := map[string]interface{}{
		"audio_base64":     huge,
		"audio_format":     "mp3_44100_128",
		"audio_size_bytes": len(huge),
		"success":          true,
	}

	f := &Flow{}
	line := captureStdout(func() {
		f.emitNodeEvent("n1", "elevenlabs/text_to_speech", "Text to Speech", "success", 1234, "", nil, outputs)
	})

	// The emitted line must stay well under the runner's 1MB scanner buffer.
	Expect(len(line)).To(BeNumerically("<", 64<<10),
		"emitted __NODE__ line must be small enough for the runner to read")

	Expect(line).To(HavePrefix("__NODE__:"))
	payload := strings.TrimSpace(strings.TrimPrefix(line, "__NODE__:"))

	var evt map[string]interface{}
	Expect(json.Unmarshal([]byte(payload), &evt)).To(Succeed())

	emittedOutputs, ok := evt["outputs"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	emittedAudio, ok := emittedOutputs["audio_base64"].(string)
	Expect(ok).To(BeTrue())
	Expect(len(emittedAudio)).To(BeNumerically("<", maxEventStringBytes+64))
	Expect(emittedAudio).To(ContainSubstring("truncated"))

	// Small values pass through untouched, including non-strings.
	Expect(emittedOutputs["audio_format"]).To(Equal("mp3_44100_128"))
	Expect(emittedOutputs["success"]).To(Equal(true))

	// Critically: the caller's map is never mutated — downstream nodes and
	// recordNodeResult must still see the full, untruncated value.
	Expect(outputs["audio_base64"]).To(Equal(huge))
	Expect(len(outputs["audio_base64"].(string))).To(Equal(10 << 20))
}

func TestTruncateEventValues(t *testing.T) {
	RegisterTestingT(t)

	Expect(truncateEventValues(nil)).To(BeNil())

	small := strings.Repeat("x", 100)
	in := map[string]interface{}{
		"small":  small,
		"number": 42,
		"flag":   false,
		"big":    strings.Repeat("y", maxEventStringBytes+1),
	}
	out := truncateEventValues(in)

	Expect(out["small"]).To(Equal(small))
	Expect(out["number"]).To(Equal(42))
	Expect(out["flag"]).To(Equal(false))
	Expect(out["big"].(string)).To(ContainSubstring("truncated"))
	Expect(len(out["big"].(string))).To(BeNumerically("<", maxEventStringBytes+64))

	// Original map untouched.
	Expect(len(in["big"].(string))).To(Equal(maxEventStringBytes + 1))
}

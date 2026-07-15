package video_common

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestTail(t *testing.T) {
	RegisterTestingT(t)
	Expect(Tail("short", 400)).To(Equal("short"))
	Expect(Tail(strings.Repeat("x", 500), 10)).To(Equal("…" + strings.Repeat("x", 10)))
	Expect(Tail("  trimmed  ", 400)).To(Equal("trimmed"))
}

func TestOptionalFloat(t *testing.T) {
	RegisterTestingT(t)
	inputs := []*core.Connection{
		{Name: "f", Type: core.ConnectionTypeString, Value: float64(2.5)},
		{Name: "i", Type: core.ConnectionTypeInteger, Value: 7},
		{Name: "s", Type: core.ConnectionTypeString, Value: "3.25"},
	}
	Expect(OptionalFloat("f", 0, inputs)).To(Equal(2.5))
	Expect(OptionalFloat("i", 0, inputs)).To(Equal(7.0))
	Expect(OptionalFloat("s", 0, inputs)).To(Equal(3.25))
	Expect(OptionalFloat("missing", 9, inputs)).To(Equal(9.0))
}

func TestLimitedBuffer_Caps(t *testing.T) {
	RegisterTestingT(t)
	lb := &limitedBuffer{cap: 10}
	n, err := lb.Write([]byte(strings.Repeat("a", 100)))
	Expect(err).To(BeNil())
	Expect(n).To(Equal(100)) // reports full length (no short write to ffmpeg)
	Expect(lb.String()).To(HaveLen(10))
}

func TestResolveBinary_MissingEnvOverride(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("FLOMATION_FFMPEG_PATH", "/nonexistent/ffmpeg-xyz")
	_, err := FFmpegPath()
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("does not exist"))
}

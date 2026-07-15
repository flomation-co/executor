package transcode

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestFormats_ValidCombosOnly(t *testing.T) {
	RegisterTestingT(t)
	// Every offered format maps to a valid container + codec pair.
	for _, key := range []string{"mp4-h264", "mp4-h265", "webm-vp9", "mkv-h265"} {
		spec, ok := formats[key]
		Expect(ok).To(BeTrue(), "missing format %q", key)
		Expect(spec.container).ToNot(BeEmpty())
		Expect(spec.vcodec).ToNot(BeEmpty())
		Expect(spec.acodec).ToNot(BeEmpty())
	}
	// WebM must use a WebM-legal codec pair (not h264/aac).
	Expect(formats["webm-vp9"].vcodec).To(Equal("libvpx-vp9"))
	Expect(formats["webm-vp9"].acodec).To(Equal("libopus"))
}

func TestTranscode_UnknownFormatRejected(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "video", Type: core.ConnectionTypeString, Value: "flo:file:x.mp4"},
		{Name: "format", Type: core.ConnectionTypeString, Value: "bogus"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["tool_result"]).To(ContainSubstring("unknown format"))
}

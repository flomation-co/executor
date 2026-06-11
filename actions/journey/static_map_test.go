package journey_common

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestDecodePolyline(t *testing.T) {
	RegisterTestingT(t)

	// Canonical Google example: "_p~iF~ps|U_ulLnnqC_mqNvxq`@" decodes to:
	// (38.5, -120.2), (40.7, -120.95), (43.252, -126.453)
	coords := DecodePolyline("_p~iF~ps|U_ulLnnqC_mqNvxq`@")
	Expect(coords).To(HaveLen(3))
	Expect(coords[0].Lat).To(BeNumerically("~", 38.5, 1e-5))
	Expect(coords[0].Lng).To(BeNumerically("~", -120.2, 1e-5))
	Expect(coords[2].Lat).To(BeNumerically("~", 43.252, 1e-5))
	Expect(coords[2].Lng).To(BeNumerically("~", -126.453, 1e-5))
}

func TestDecodePolylineEmpty(t *testing.T) {
	RegisterTestingT(t)
	Expect(DecodePolyline("")).To(BeNil())
}

func TestGoogleRenderStaticMap(t *testing.T) {
	RegisterTestingT(t)

	pngBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	client, tr := stubClient(200, string(pngBytes))
	p, _ := NewProviderWithClient("google", "test-key", client)

	img, mime, err := p.RenderStaticMap(StaticMapParams{
		Polyline: "_p~iF~ps|U_ulLnnqC",
		Width:    640,
		Height:   480,
		Zoom:     10,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(img).To(HaveLen(len(pngBytes)))
	Expect(mime).ToNot(BeEmpty())

	q := tr.last.URL.Query()
	Expect(q.Get("size")).To(Equal("640x480"))
	Expect(q.Get("zoom")).To(Equal("10"))
	Expect(strings.HasPrefix(q.Get("path"), "color:0x0000ffff|weight:4|enc:")).To(BeTrue())
	Expect(q.Get("key")).To(Equal("test-key"))
}

func TestGoogleRenderStaticMapRequiresPolyline(t *testing.T) {
	RegisterTestingT(t)

	client, _ := stubClient(200, "")
	p, _ := NewProviderWithClient("google", "test-key", client)

	_, _, err := p.RenderStaticMap(StaticMapParams{Polyline: "  "})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("polyline"))
}

func TestMapboxRenderStaticMap(t *testing.T) {
	RegisterTestingT(t)

	pngBytes := []byte{0x89, 0x50, 0x4e, 0x47}
	client, tr := stubClient(200, string(pngBytes))
	p, _ := NewProviderWithClient("mapbox", "test-key", client)

	img, _, err := p.RenderStaticMap(StaticMapParams{
		Polyline: "_p~iF~ps|U",
		Width:    600,
		Height:   400,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(img).To(HaveLen(len(pngBytes)))

	// Mapbox bakes overlays into the URL path
	Expect(strings.Contains(tr.last.URL.Path, "/styles/v1/mapbox/streets-v12/static/")).To(BeTrue())
	Expect(strings.Contains(tr.last.URL.Path, "path-4+0000ff")).To(BeTrue())
	Expect(strings.Contains(tr.last.URL.Path, "/auto/600x400")).To(BeTrue())
}

package video_watermark

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestOverlayXY(t *testing.T) {
	RegisterTestingT(t)
	Expect(overlayXY("Center", 10)).To(Equal("(main_w-overlay_w)/2:(main_h-overlay_h)/2"))
	Expect(overlayXY("NorthWest", 5)).To(Equal("5:5"))
	Expect(overlayXY("NorthEast", 8)).To(Equal("main_w-overlay_w-8:8"))
	Expect(overlayXY("SouthWest", 8)).To(Equal("8:main_h-overlay_h-8"))
	Expect(overlayXY("SouthEast", 12)).To(Equal("main_w-overlay_w-12:main_h-overlay_h-12"))
}

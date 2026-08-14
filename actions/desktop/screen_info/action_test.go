package screen_info

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestParseScreenInfo(t *testing.T) {
	RegisterTestingT(t)

	w, h, win, err := ParseScreenInfo("1280 800\nGoogle Chrome\n")
	Expect(err).To(BeNil())
	Expect(w).To(Equal(int64(1280)))
	Expect(h).To(Equal(int64(800)))
	Expect(win).To(Equal("Google Chrome"))
}

// A bare desktop with nothing focused reports geometry and no title. That is a
// normal state, not a failure — treating it as one would make the action
// useless exactly when an agent most needs to know what it is looking at.
func TestParseScreenInfo_MissingActiveWindow(t *testing.T) {
	RegisterTestingT(t)

	for _, out := range []string{"1280 800\n", "1280 800", "1280 800\n\n"} {
		w, h, win, err := ParseScreenInfo(out)
		Expect(err).To(BeNil(), "input %q", out)
		Expect(w).To(Equal(int64(1280)))
		Expect(h).To(Equal(int64(800)))
		Expect(win).To(Equal(""))
	}
}

func TestParseScreenInfo_WindowsLineEndingsAndSpacedTitle(t *testing.T) {
	RegisterTestingT(t)

	_, _, win, err := ParseScreenInfo("1920 1080\r\nFlomation — Google Chrome\r\n")
	Expect(err).To(BeNil())
	Expect(win).To(Equal("Flomation — Google Chrome"))
}

func TestParseScreenInfo_Rejects(t *testing.T) {
	RegisterTestingT(t)

	for _, out := range []string{"", "   ", "1280", "wide tall"} {
		_, _, _, err := ParseScreenInfo(out)
		Expect(err).NotTo(BeNil(), "input %q should be rejected", out)
	}
}

// The whole point of the action: a display at or under the vision long-edge
// limit is shown untouched, so screenshot coordinates are screen coordinates.
func TestScreenshotScale_NoScalingWhenSmallEnough(t *testing.T) {
	RegisterTestingT(t)

	Expect(ScreenshotScale(1280, 800)).To(Equal(1.0))
	Expect(ScreenshotScale(1440, 900)).To(Equal(1.0))
	Expect(ScreenshotScale(1512, 945)).To(Equal(1.0))
	Expect(ScreenshotScale(1568, 1568)).To(Equal(1.0)) // exactly at the limit
}

// Above the limit the image is shrunk, so a coordinate read off it is in a
// different space from the one a click consumes. This is the silent failure the
// action exists to surface.
func TestScreenshotScale_ShrinksAboveLimit(t *testing.T) {
	RegisterTestingT(t)

	scale := ScreenshotScale(1920, 1080)
	Expect(scale).To(BeNumerically("~", 1568.0/1920.0, 1e-9))
	Expect(scale).To(BeNumerically("<", 1.0))

	// A button the model sees at x=1344 is really at x≈1646 on screen. Clicking
	// 1344 lands ~300px short, with nothing to indicate why.
	Expect(1344 * (1 / scale)).To(BeNumerically("~", 1646, 1))
}

// The long edge governs, whichever way round the screen is.
func TestScreenshotScale_UsesLongEdge(t *testing.T) {
	RegisterTestingT(t)

	Expect(ScreenshotScale(1080, 1920)).To(Equal(ScreenshotScale(1920, 1080)))
	Expect(ScreenshotScale(2000, 100)).To(BeNumerically("~", 1568.0/2000.0, 1e-9))
}

package graphics_common

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
)

// Fonts are EMBEDDED so rendering is deterministic on every runner — no system
// fonts, no libfreetype, no runtime download. Poppins is a Flomation brand font
// (SIL Open Font License; see fonts/Poppins-OFL.txt).
//
//go:embed fonts/Poppins-Regular.ttf fonts/Poppins-SemiBold.ttf fonts/Poppins-Bold.ttf fonts/Poppins-OFL.txt
var fontFS embed.FS

var fontFiles = map[string]string{
	"poppins-regular":  "fonts/Poppins-Regular.ttf",
	"poppins-semibold": "fonts/Poppins-SemiBold.ttf",
	"poppins-bold":     "fonts/Poppins-Bold.ttf",
}

// FontOptions is the ConnectionOption list every graphics action reuses for its
// font dropdown (inline so the manifest generator resolves it — must be copied
// into the action's Inputs literal, not referenced).
//
// Kept here only for reference; actions inline the equivalent options.

// FontFace loads an embedded font at the given pixel size (DPI 72 → 1pt == 1px).
func FontFace(name string, sizePx float64) (font.Face, error) {
	path, ok := fontFiles[strings.ToLower(name)]
	if !ok {
		path = fontFiles["poppins-semibold"]
	}
	b, err := fontFS.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := opentype.Parse(b)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: sizePx, DPI: 72, Hinting: font.HintingFull})
}

// RenderVideo draws each frame with draw(dc, t) — t is the frame time in seconds —
// to a transparent gg canvas, saves the frames as PNGs, then assembles them into an
// animated PNG (APNG). APNG is chosen deliberately over a video container:
//   - full 8-bit alpha (crisp anti-aliased text), preserved losslessly;
//   - written by ffmpeg's BUILT-IN apng muxer — no libvpx/libx264, so it works with
//     any stock ffmpeg (WebM/VP8/VP9 alpha silently drops the alpha channel on many
//     builds — including Homebrew's — which produced opaque or unplayable output);
//   - web-native: browsers animate APNG in an <img>, so the editor previews it inline
//     and a download opens anywhere (a qtrle .mov played in neither);
//   - still composites downstream: ffmpeg reads the APNG's alpha in the Overlay action.
// Returns the output path.
func RenderVideo(ctx context.Context, flow *core.Flow, width, height, fps int, duration float64, draw func(dc *gg.Context, t float64)) (string, error) {
	if width < 2 {
		width = 2
	}
	if height < 2 {
		height = 2
	}
	if fps < 1 {
		fps = 30
	}
	frames := int(duration * float64(fps))
	if frames < 1 {
		frames = 1
	}

	dir, err := flow.MediaScratchFile("") // use the unique path as a frames dir
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	for i := 0; i < frames; i++ {
		t := float64(i) / float64(fps)
		dc := gg.NewContext(width, height) // transparent until drawn on
		draw(dc, t)
		if err := dc.SavePNG(filepath.Join(dir, fmt.Sprintf("f_%05d.png", i))); err != nil {
			return "", err
		}
	}

	out, err := flow.MediaScratchFile("apng")
	if err != nil {
		return "", err
	}
	// APNG keeps a full alpha channel (transparent for overlay) and is written by
	// ffmpeg's built-in muxer, so this needs no external codec library. -plays 0
	// loops forever; -f apng forces the muxer regardless of the output extension.
	stderr, err := vc.RunFFmpeg(ctx, "-y", "-framerate", strconv.Itoa(fps),
		"-i", filepath.Join(dir, "f_%05d.png"), "-plays", "0", "-f", "apng", out)
	if err != nil {
		return "", fmt.Errorf("assemble frames: %v: %s", err, vc.Tail(stderr, 300))
	}
	return out, nil
}

// ── Colour ──

// SetColour applies a hex ("#rgb"/"#rrggbb") or a few named colours to dc, with an
// explicit alpha (0..1).
func SetColour(dc *gg.Context, spec string, alpha float64) {
	r, g, b := parseColour(spec)
	dc.SetRGBA(r, g, b, clamp01(alpha))
}

func parseColour(spec string) (r, g, b float64) {
	s := strings.TrimSpace(strings.ToLower(spec))
	switch s {
	case "white", "":
		return 1, 1, 1
	case "black":
		return 0, 0, 0
	case "flomation-purple":
		s = "#460070"
	case "flomation-teal":
		s = "#00aa9c"
	}
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return 1, 1, 1 // default white
	}
	ri, _ := strconv.ParseInt(s[0:2], 16, 0)
	gi, _ := strconv.ParseInt(s[2:4], 16, 0)
	bi, _ := strconv.ParseInt(s[4:6], 16, 0)
	return float64(ri) / 255, float64(gi) / 255, float64(bi) / 255
}

// ── Easing / maths ──

func EaseOutCubic(t float64) float64 { t -= 1; return t*t*t + 1 }
func Lerp(a, b, t float64) float64   { return a + (b-a)*t }
func clamp01(v float64) float64      { return Clamp(v, 0, 1) }

func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ── Input + result helpers ──

func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return strings.TrimSpace(*c.String())
}

func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

func OptionalStringDefault(name, def string, inputs []*core.Connection) string {
	if v := OptionalString(name, inputs); v != "" {
		return v
	}
	return def
}

func OptionalInt(name string, def int, inputs []*core.Connection) int {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return def
	}
	switch v := c.Value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func OptionalFloat(name string, def float64, inputs []*core.Connection) float64 {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return def
	}
	switch v := c.Value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return n
		}
	}
	return def
}

func ErrResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"success":     false,
		"error":       msg,
	}, nil
}

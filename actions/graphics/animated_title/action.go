// Package animated_title generates an animated title as a transparent video (for
// overlaying onto a base video). Text is rendered in pure Go with embedded fonts.
package animated_title

import (
	"context"
	"fmt"
	"os"

	"github.com/fogleman/gg"

	core "flomation.app/automate/executor"
	gc "flomation.app/automate/executor/actions/graphics"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Animated Title"
	Description  = "Generate an animated text title as a transparent video for overlaying"
	Website      = "https://www.flomation.co"
	Icon         = "pen"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "text", Type: core.ConnectionTypeText, Label: "Title text", Placeholder: "Your title", Required: true},
	{
		Name: "font", Type: core.ConnectionTypeString, Label: "Font", Value: "poppins-bold",
		Options: []core.ConnectionOption{
			{Name: "Poppins Bold", Value: "poppins-bold"},
			{Name: "Poppins SemiBold", Value: "poppins-semibold"},
			{Name: "Poppins Regular", Value: "poppins-regular"},
		},
	},
	{Name: "font_size", Type: core.ConnectionTypeInteger, Label: "Font size (px)", Value: 80},
	{Name: "colour", Type: core.ConnectionTypeString, Label: "Colour", Value: "#ffffff", Placeholder: "#ffffff, flomation-teal"},
	{
		Name: "animation", Type: core.ConnectionTypeString, Label: "Animation", Value: "fade",
		Options: []core.ConnectionOption{
			{Name: "Fade", Value: "fade"},
			{Name: "Slide from right", Value: "slide_left"},
			{Name: "Slide from left", Value: "slide_right"},
			{Name: "Rise up", Value: "rise"},
		},
	},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)", Value: 1280},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px)", Value: 300},
	{Name: "duration_seconds", Type: core.ConnectionTypeString, Label: "Duration (seconds)", Value: "4"},
	{Name: "fps", Type: core.ConnectionTypeInteger, Label: "Frames per second", Value: 30},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Transparent animation (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// titleAnim returns the (dx, dy, alpha) for the text at time t given the animation,
// the total duration and the canvas size. Pure — unit-tested without ffmpeg.
func titleAnim(animation string, t, dur, w, h float64) (dx, dy, alpha float64) {
	const inDur = 0.5
	alpha = 1
	if t < inDur {
		alpha = gc.EaseOutCubic(gc.Clamp(t/inDur, 0, 1))
	} else if t > dur-inDur {
		alpha = gc.EaseOutCubic(gc.Clamp((dur-t)/inDur, 0, 1))
	}
	prog := gc.EaseOutCubic(gc.Clamp(t/inDur, 0, 1)) // 0→1 over the intro
	switch animation {
	case "slide_left":
		dx = w * (1 - prog) // from off the right edge to centred
	case "slide_right":
		dx = -w * (1 - prog)
	case "rise":
		dy = (h * 0.15) * (1 - prog)
	}
	return dx, dy, alpha
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	text, err := gc.RequiredString("text", inputs)
	if err != nil {
		return gc.ErrResult(err.Error())
	}
	fontName := gc.OptionalStringDefault("font", "poppins-bold", inputs)
	fontSize := gc.OptionalInt("font_size", 80, inputs)
	if fontSize < 4 {
		fontSize = 4
	}
	colour := gc.OptionalStringDefault("colour", "#ffffff", inputs)
	animation := gc.OptionalStringDefault("animation", "fade", inputs)
	width := gc.OptionalInt("width", 1280, inputs)
	height := gc.OptionalInt("height", 300, inputs)
	duration := gc.OptionalFloat("duration_seconds", 4, inputs)
	if duration <= 0 {
		duration = 4
	}
	fps := gc.OptionalInt("fps", 30, inputs)

	face, err := gc.FontFace(fontName, float64(fontSize))
	if err != nil {
		return gc.ErrResult("could not load the font: " + err.Error())
	}

	draw := func(dc *gg.Context, t float64) {
		dx, dy, alpha := titleAnim(animation, t, duration, float64(width), float64(height))
		dc.SetFontFace(face)
		gc.SetColour(dc, colour, alpha)
		dc.DrawStringAnchored(text, float64(width)/2+dx, float64(height)/2+dy, 0.5, 0.5)
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), 300e9) // 300s
	defer cancel()

	outPath, err := gc.RenderVideo(ctx, flow, width, height, fps, duration, draw)
	if err != nil {
		return gc.ErrResult(err.Error())
	}

	ref, err := flow.EmitMediaFile(outPath)
	if err != nil {
		return gc.ErrResult(err.Error())
	}
	var size int64
	if fi, e := os.Stat(outPath); e == nil {
		size = fi.Size()
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Rendered an animated title (%s, %.1fs, %d bytes).", animation, duration, size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

// Package lower_third generates an animated lower-third (title + subtitle bar that
// slides in) as a transparent video for overlaying. Pure-Go text rendering.
package lower_third

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
	Name         = "Animated Lower-Third"
	Description  = "Generate an animated lower-third (title + subtitle bar) as a transparent video"
	Website      = "https://www.flomation.co"
	Icon         = "align-left"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "Jane Doe", Required: true},
	{Name: "subtitle", Type: core.ConnectionTypeString, Label: "Subtitle (optional)", Placeholder: "Head of Product"},
	{Name: "accent_colour", Type: core.ConnectionTypeColour, Label: "Accent colour", Value: "#00aa9c", Placeholder: "#00aa9c"},
	{Name: "text_colour", Type: core.ConnectionTypeColour, Label: "Text colour", Value: "#ffffff"},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)", Value: 1280},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px)", Value: 200},
	{Name: "duration_seconds", Type: core.ConnectionTypeString, Label: "Duration (seconds)", Value: "5"},
	{Name: "fps", Type: core.ConnectionTypeInteger, Label: "Frames per second", Value: 30},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Transparent animation (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// slideOffset returns the horizontal offset: off-screen-left → 0 during the intro,
// holds, then back off-screen during the outro. Pure — unit-tested.
func slideOffset(t, dur, w float64) float64 {
	const anim = 0.5
	if t < anim {
		return -w * (1 - gc.EaseOutCubic(gc.Clamp(t/anim, 0, 1)))
	}
	if t > dur-anim {
		return -w * (1 - gc.EaseOutCubic(gc.Clamp((dur-t)/anim, 0, 1)))
	}
	return 0
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	title, err := gc.RequiredString("title", inputs)
	if err != nil {
		return gc.ErrResult(err.Error())
	}
	subtitle := gc.OptionalString("subtitle", inputs)
	accent := gc.OptionalStringDefault("accent_colour", "#00aa9c", inputs)
	textColour := gc.OptionalStringDefault("text_colour", "#ffffff", inputs)
	width := gc.OptionalInt("width", 1280, inputs)
	height := gc.OptionalInt("height", 200, inputs)
	duration := gc.OptionalFloat("duration_seconds", 5, inputs)
	if duration <= 0 {
		duration = 5
	}
	fps := gc.OptionalInt("fps", 30, inputs)
	w, h := float64(width), float64(height)

	titleFace, err := gc.FontFace("poppins-bold", h*0.34)
	if err != nil {
		return gc.ErrResult("could not load the font: " + err.Error())
	}
	subFace, err := gc.FontFace("poppins-regular", h*0.22)
	if err != nil {
		return gc.ErrResult("could not load the font: " + err.Error())
	}

	draw := func(dc *gg.Context, t float64) {
		x := slideOffset(t, duration, w)
		barW := w * 0.6
		// Accent stripe + translucent dark bar.
		gc.SetColour(dc, accent, 1)
		dc.DrawRectangle(x, 0, h*0.06, h)
		dc.Fill()
		dc.SetRGBA(0.08, 0.08, 0.11, 0.82)
		dc.DrawRectangle(x+h*0.06, 0, barW, h)
		dc.Fill()
		// Text.
		textX := x + h*0.06 + h*0.25
		gc.SetColour(dc, textColour, 1)
		if subtitle != "" {
			dc.SetFontFace(titleFace)
			dc.DrawStringAnchored(title, textX, h*0.36, 0, 0.5)
			dc.SetFontFace(subFace)
			gc.SetColour(dc, accent, 1)
			dc.DrawStringAnchored(subtitle, textX, h*0.68, 0, 0.5)
		} else {
			dc.SetFontFace(titleFace)
			dc.DrawStringAnchored(title, textX, h*0.5, 0, 0.5)
		}
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), 300e9)
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
		"tool_result": fmt.Sprintf("Rendered a lower-third (%.1fs, %d bytes).", duration, size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

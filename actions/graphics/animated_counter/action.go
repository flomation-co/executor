// Package animated_counter generates a number counting from A to B with easing, as
// a transparent video for overlaying. Pure-Go text rendering.
package animated_counter

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/fogleman/gg"

	core "flomation.app/automate/executor"
	gc "flomation.app/automate/executor/actions/graphics"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Animated Counter"
	Description  = "Generate a number counting from one value to another as a transparent video"
	Website      = "https://www.flomation.co"
	Icon         = "hashtag"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "from", Type: core.ConnectionTypeString, Label: "From", Value: "0", Placeholder: "0"},
	{Name: "to", Type: core.ConnectionTypeString, Label: "To", Value: "100", Placeholder: "100"},
	{Name: "decimals", Type: core.ConnectionTypeInteger, Label: "Decimal places", Value: 0},
	{Name: "prefix", Type: core.ConnectionTypeString, Label: "Prefix", Placeholder: "£"},
	{Name: "suffix", Type: core.ConnectionTypeString, Label: "Suffix", Placeholder: "%"},
	{
		Name: "font", Type: core.ConnectionTypeString, Label: "Font", Value: "poppins-bold",
		Options: []core.ConnectionOption{
			{Name: "Poppins Bold", Value: "poppins-bold"},
			{Name: "Poppins SemiBold", Value: "poppins-semibold"},
			{Name: "Poppins Regular", Value: "poppins-regular"},
		},
	},
	{Name: "font_size", Type: core.ConnectionTypeInteger, Label: "Font size (px)", Value: 120},
	{Name: "colour", Type: core.ConnectionTypeColour, Label: "Colour", Value: "#ffffff"},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)", Value: 700},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px)", Value: 300},
	{Name: "duration_seconds", Type: core.ConnectionTypeString, Label: "Duration (seconds)", Value: "2"},
	{Name: "fps", Type: core.ConnectionTypeInteger, Label: "Frames per second", Value: 30},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Transparent animation (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	from := gc.OptionalFloat("from", 0, inputs)
	to := gc.OptionalFloat("to", 100, inputs)
	decimals := gc.OptionalInt("decimals", 0, inputs)
	if decimals < 0 {
		decimals = 0
	}
	prefix := gc.OptionalString("prefix", inputs)
	suffix := gc.OptionalString("suffix", inputs)
	fontName := gc.OptionalStringDefault("font", "poppins-bold", inputs)
	fontSize := gc.OptionalInt("font_size", 120, inputs)
	if fontSize < 4 {
		fontSize = 4
	}
	colour := gc.OptionalStringDefault("colour", "#ffffff", inputs)
	width := gc.OptionalInt("width", 700, inputs)
	height := gc.OptionalInt("height", 300, inputs)
	duration := gc.OptionalFloat("duration_seconds", 2, inputs)
	if duration <= 0 {
		duration = 2
	}
	fps := gc.OptionalInt("fps", 30, inputs)

	face, err := gc.FontFace(fontName, float64(fontSize))
	if err != nil {
		return gc.ErrResult("could not load the font: " + err.Error())
	}

	draw := func(dc *gg.Context, t float64) {
		p := gc.EaseOutCubic(gc.Clamp(t/duration, 0, 1))
		val := gc.Lerp(from, to, p)
		label := prefix + strconv.FormatFloat(val, 'f', decimals, 64) + suffix
		dc.SetFontFace(face)
		gc.SetColour(dc, colour, 1)
		dc.DrawStringAnchored(label, float64(width)/2, float64(height)/2, 0.5, 0.5)
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
		"tool_result": fmt.Sprintf("Rendered a counter %g→%g (%.1fs, %d bytes).", from, to, duration, size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

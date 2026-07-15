// Package waveform renders an audio track as a waveform image, backed by ffmpeg's
// showwavespic filter.
package waveform

import (
	"context"
	"fmt"
	"os"

	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Audio Waveform Image"
	Description  = "Render an audio track as a waveform PNG image"
	Website      = "https://www.flomation.co"
	Icon         = "chart-line"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "audio", Type: core.ConnectionTypeString, Label: "Audio (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)", Value: 1000},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px)", Value: 200},
	{Name: "colour", Type: core.ConnectionTypeString, Label: "Colour", Value: "#00aa9c", Placeholder: "#00aa9c"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Waveform image (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	audio, err := vc.RequiredString("audio", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	w := vc.OptionalInt("width", 1000, inputs)
	h := vc.OptionalInt("height", 200, inputs)
	if w < 1 {
		w = 1000
	}
	if h < 1 {
		h = 200
	}
	colour := vc.OptionalStringDefault("colour", "#00aa9c", inputs)

	inPath, _, err := flow.ResolveToLocalFile(audio)
	if err != nil {
		return vc.ErrResult("could not read the input audio: " + err.Error())
	}
	outPath, err := flow.MediaScratchFile("png")
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.DefaultTimeout)
	defer cancel()

	filter := fmt.Sprintf("showwavespic=s=%dx%d:colors=%s", w, h, colour)
	stderr, err := vc.RunFFmpeg(ctx, "-y", "-i", inPath,
		"-filter_complex", filter, "-frames:v", "1", outPath)
	if err != nil {
		return vc.ErrResult(fmt.Sprintf("ffmpeg failed: %v: %s", err, vc.Tail(stderr, 400)))
	}

	ref, err := flow.EmitMediaFile(outPath)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	var size int64
	if fi, e := os.Stat(outPath); e == nil {
		size = fi.Size()
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Rendered a %dx%d waveform (%d bytes).", w, h, size),
		"image":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

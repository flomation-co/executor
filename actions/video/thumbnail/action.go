// Package thumbnail captures a single frame from a video as an image. ffmpeg-backed.
package thumbnail

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
	Name         = "Create Thumbnail"
	Description  = "Capture a single frame from a video as a PNG or JPEG image"
	Website      = "https://www.flomation.co"
	Icon         = "image"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "timestamp_seconds", Type: core.ConnectionTypeString, Label: "Timestamp (seconds)", Value: "1", Placeholder: "1"},
	{
		Name: "format", Type: core.ConnectionTypeString, Label: "Image format", Value: "png",
		Options: []core.ConnectionOption{
			{Name: "PNG", Value: "png"},
			{Name: "JPEG", Value: "jpg"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (media reference)"},
	{Name: "format", Type: core.ConnectionTypeString, Label: "Image format"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	ts := vc.OptionalStringDefault("timestamp_seconds", "1", inputs)
	format := vc.OptionalStringDefault("format", "png", inputs)

	inPath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the input video: " + err.Error())
	}
	outPath, err := flow.MediaScratchFile(format)
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.DefaultTimeout)
	defer cancel()

	// Seek before -i for a fast seek; grab exactly one frame.
	stderr, err := vc.RunFFmpeg(ctx, "-y", "-ss", ts, "-i", inPath, "-frames:v", "1", outPath)
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
		"tool_result": fmt.Sprintf("Captured a %s thumbnail at %ss (%d bytes).", format, ts, size),
		"image":       ref,
		"format":      format,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

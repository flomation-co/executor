// Package ken_burns animates a still image with a slow zoom/pan (the "Ken Burns"
// effect), producing a video clip. ffmpeg zoompan-backed.
package ken_burns

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
	Name         = "Ken Burns (Animate Image)"
	Description  = "Animate a still image with a slow zoom and pan, producing a video clip"
	Website      = "https://www.flomation.co"
	Icon         = "magnifying-glass"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "duration_seconds", Type: core.ConnectionTypeString, Label: "Duration (seconds)", Value: "5", Placeholder: "5"},
	{
		Name: "zoom", Type: core.ConnectionTypeString, Label: "Zoom", Value: "in",
		Options: []core.ConnectionOption{
			{Name: "Zoom in", Value: "in"},
			{Name: "Zoom out", Value: "out"},
		},
	},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)", Value: 1280},
	{Name: "fps", Type: core.ConnectionTypeInteger, Label: "Frames per second", Value: 25},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	image, err := vc.RequiredString("image", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	duration := vc.OptionalFloat("duration_seconds", 5, inputs)
	if duration <= 0 {
		duration = 5
	}
	zoom := vc.OptionalStringDefault("zoom", "in", inputs)
	width := vc.OptionalInt("width", 1280, inputs)
	if width < 2 {
		width = 1280
	}
	if width%2 != 0 {
		width++
	}
	height := width * 9 / 16
	if height%2 != 0 {
		height++
	}
	fps := vc.OptionalInt("fps", 25, inputs)
	if fps < 1 {
		fps = 25
	}
	frames := int(duration * float64(fps))
	if frames < 1 {
		frames = 1
	}

	inPath, _, err := flow.ResolveToLocalFile(image)
	if err != nil {
		return vc.ErrResult("could not read the input image: " + err.Error())
	}
	outPath, err := flow.MediaScratchFile("mp4")
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.MaxTimeout)
	defer cancel()

	// Upscale first so the zoom stays smooth, then zoompan across the frames.
	// x/y keep the (zoomed) frame centred.
	var z string
	if zoom == "out" {
		z = "if(eq(on,0),1.5,max(zoom-0.0015,1.0))"
	} else {
		z = "min(zoom+0.0015,1.5)"
	}
	filter := fmt.Sprintf("scale=%d:-1,zoompan=z='%s':d=%d:x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':s=%dx%d:fps=%d",
		width*4, z, frames, width, height, fps)

	stderr, err := vc.RunFFmpeg(ctx, "-y", "-loop", "1", "-i", inPath,
		"-vf", filter, "-t", fmt.Sprintf("%.2f", duration), "-pix_fmt", "yuv420p", outPath)
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
		"tool_result": fmt.Sprintf("Animated the image (%.1fs, zoom %s, %d bytes).", duration, zoom, size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

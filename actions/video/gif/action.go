// Package gif converts a video (or a clip of one) into an animated GIF, backed by
// ffmpeg with a two-stage palette for good quality.
package gif

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
	Name         = "Video to GIF"
	Description  = "Convert a video, or a portion of it, into an animated GIF"
	Website      = "https://www.flomation.co"
	Icon         = "film"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "start_seconds", Type: core.ConnectionTypeString, Label: "Start (seconds)", Value: "0", Placeholder: "0"},
	{Name: "duration_seconds", Type: core.ConnectionTypeString, Label: "Duration (seconds, blank = whole)", Placeholder: "5"},
	{Name: "fps", Type: core.ConnectionTypeString, Label: "Frames per second", Value: "12", Placeholder: "12"},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)", Value: 480},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "gif", Type: core.ConnectionTypeString, Label: "GIF (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	start := vc.OptionalStringDefault("start_seconds", "0", inputs)
	duration := vc.OptionalString("duration_seconds", inputs)
	fps := vc.OptionalStringDefault("fps", "12", inputs)
	width := vc.OptionalInt("width", 480, inputs)
	if width < 1 {
		width = 480
	}

	inPath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the input video: " + err.Error())
	}
	outPath, err := flow.MediaScratchFile("gif")
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.DefaultTimeout)
	defer cancel()

	// One-pass palettegen/paletteuse via split — far better quality than the
	// default 216-colour GIF palette.
	filter := fmt.Sprintf("fps=%s,scale=%d:-1:flags=lanczos,split[a][b];[a]palettegen[p];[b][p]paletteuse", fps, width)
	args := []string{"-y"}
	if start != "" && start != "0" {
		args = append(args, "-ss", start)
	}
	if duration != "" {
		args = append(args, "-t", duration)
	}
	args = append(args, "-i", inPath, "-filter_complex", filter, outPath)

	stderr, err := vc.RunFFmpeg(ctx, args...)
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
		"tool_result": fmt.Sprintf("Made a GIF at %s fps, %dpx wide (%d bytes).", fps, width, size),
		"gif":         ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

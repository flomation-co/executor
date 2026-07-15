// Package trim cuts a clip from a video by start time and duration. ffmpeg-backed.
package trim

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Trim Video"
	Description  = "Cut a clip from a video by start time and duration"
	Website      = "https://www.flomation.co"
	Icon         = "clock"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "start_seconds", Type: core.ConnectionTypeString, Label: "Start (seconds)", Value: "0", Placeholder: "0"},
	{Name: "duration_seconds", Type: core.ConnectionTypeString, Label: "Duration (seconds)", Placeholder: "10", Required: true},
	{Name: "reencode", Type: core.ConnectionTypeBoolean, Label: "Re-encode (slower, frame-accurate)", Value: false},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file reference)"},
	{Name: "duration_seconds", Type: core.ConnectionTypeString, Label: "Duration (seconds)"},
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
	duration, err := vc.RequiredString("duration_seconds", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	reencode := vc.OptionalBool("reencode", false, inputs)

	inPath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the input video: " + err.Error())
	}
	ext := filepath.Ext(inPath)
	if ext == "" {
		ext = ".mp4"
	}
	outPath, err := flow.MediaScratchFile(ext)
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.DefaultTimeout)
	defer cancel()

	// -ss before -i seeks fast; -t bounds the clip. Stream-copy by default so it's
	// fast and lossless; re-encode only when the caller wants frame accuracy.
	args := []string{"-y", "-ss", start, "-i", inPath, "-t", duration}
	if !reencode {
		args = append(args, "-c", "copy")
	}
	args = append(args, outPath)

	stderr, err := vc.RunFFmpeg(ctx, args...)
	if err != nil {
		return vc.ErrResult(fmt.Sprintf("ffmpeg failed: %v: %s", err, vc.Tail(stderr, 400)))
	}

	ref, err := flow.EmitLocalFile(outPath)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	var size int64
	if fi, e := os.Stat(outPath); e == nil {
		size = fi.Size()
	}
	var outDur float64
	if p, e := vc.Probe(ctx, outPath); e == nil {
		outDur = p.DurationSeconds
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Trimmed to %.1fs from %ss (%d bytes).", outDur, start, size),
		"video":            ref,
		"duration_seconds": fmt.Sprintf("%.3f", outDur),
		"size_bytes":       size,
		"success":          true,
	}, nil
}

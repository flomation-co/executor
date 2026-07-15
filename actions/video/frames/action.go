// Package frames extracts still frames from a video as images, backed by ffmpeg.
package frames

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Extract Frames"
	Description  = "Extract still frames from a video as images at a chosen rate"
	Website      = "https://www.flomation.co"
	Icon         = "copy"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction

	// maxFramesCap bounds how many frames a single call can produce (each frame
	// is emitted as media, so this also bounds blob uploads).
	maxFramesCap = 200
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "fps", Type: core.ConnectionTypeString, Label: "Frames per second", Value: "1", Placeholder: "1"},
	{
		Name: "format", Type: core.ConnectionTypeString, Label: "Image format", Value: "png",
		Options: []core.ConnectionOption{
			{Name: "PNG", Value: "png"},
			{Name: "JPEG", Value: "jpg"},
		},
	},
	{Name: "max_frames", Type: core.ConnectionTypeInteger, Label: "Maximum frames", Value: 30},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "frames", Type: core.ConnectionTypeObject, Label: "Frames (media references)"},
	{Name: "first_frame", Type: core.ConnectionTypeString, Label: "First frame (media reference)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Frame count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	fps := vc.OptionalStringDefault("fps", "1", inputs)
	format := vc.OptionalStringDefault("format", "png", inputs)
	maxFrames := vc.OptionalInt("max_frames", 30, inputs)
	if maxFrames < 1 {
		maxFrames = 1
	} else if maxFrames > maxFramesCap {
		maxFrames = maxFramesCap
	}

	inPath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the input video: " + err.Error())
	}

	// Frames land in their own scratch subdirectory.
	framesDir, err := flow.MediaScratchFile("")
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	if err := os.MkdirAll(framesDir, 0o700); err != nil {
		return vc.ErrResult(err.Error())
	}
	pattern := filepath.Join(framesDir, "frame_%04d."+format)

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.DefaultTimeout)
	defer cancel()

	stderr, err := vc.RunFFmpeg(ctx, "-y", "-i", inPath,
		"-vf", "fps="+fps, "-frames:v", fmt.Sprintf("%d", maxFrames), pattern)
	if err != nil {
		return vc.ErrResult(fmt.Sprintf("ffmpeg failed: %v: %s", err, vc.Tail(stderr, 400)))
	}

	matches, _ := filepath.Glob(filepath.Join(framesDir, "frame_*."+format))
	sort.Strings(matches)
	refs := make([]string, 0, len(matches))
	for _, m := range matches {
		ref, e := flow.EmitMediaFile(m)
		if e != nil {
			return vc.ErrResult("could not emit a frame: " + e.Error())
		}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return vc.ErrResult("no frames were produced — check the fps and the input video")
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Extracted %d frame(s) at %s fps.", len(refs), fps),
		"frames":      refs,
		"first_frame": refs[0],
		"count":       len(refs),
		"success":     true,
	}, nil
}

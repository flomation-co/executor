// Package fade fades a video in from / out to black, backed by ffmpeg's fade filter.
package fade

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
	Name         = "Fade Video In / Out"
	Description  = "Fade a video in from and/or out to black over a number of seconds"
	Website      = "https://www.flomation.co"
	Icon         = "film"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "fade_in_seconds", Type: core.ConnectionTypeString, Label: "Fade in (seconds, 0 = none)", Value: "1", Placeholder: "1"},
	{Name: "fade_out_seconds", Type: core.ConnectionTypeString, Label: "Fade out (seconds, 0 = none)", Value: "1", Placeholder: "1"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	fadeIn := vc.OptionalFloat("fade_in_seconds", 1, inputs)
	fadeOut := vc.OptionalFloat("fade_out_seconds", 1, inputs)
	if fadeIn <= 0 && fadeOut <= 0 {
		return vc.ErrResult("set a fade in and/or fade out duration greater than zero")
	}

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

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.MaxTimeout)
	defer cancel()

	var vf string
	if fadeIn > 0 {
		vf = fmt.Sprintf("fade=t=in:st=0:d=%.2f", fadeIn)
	}
	if fadeOut > 0 {
		probe, _ := vc.Probe(ctx, inPath)
		if probe == nil || probe.DurationSeconds <= 0 {
			return vc.ErrResult("fade out needs the video duration, which could not be read")
		}
		st := probe.DurationSeconds - fadeOut
		if st < 0 {
			st = 0
		}
		out := fmt.Sprintf("fade=t=out:st=%.2f:d=%.2f", st, fadeOut)
		if vf != "" {
			vf += "," + out
		} else {
			vf = out
		}
	}

	stderr, err := vc.RunFFmpeg(ctx, "-y", "-i", inPath, "-vf", vf, "-c:a", "copy", outPath)
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
		"tool_result": fmt.Sprintf("Faded the video (in %.1fs, out %.1fs, %d bytes).", fadeIn, fadeOut, size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

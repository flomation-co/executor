// Package scale resizes a video to a standard resolution, backed by ffmpeg.
package scale

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
	Name         = "Scale Video"
	Description  = "Resize a video to a standard resolution, keeping the aspect ratio"
	Website      = "https://www.flomation.co"
	Icon         = "expand"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "resolution", Type: core.ConnectionTypeString, Label: "Resolution", Value: "720p",
		Options: []core.ConnectionOption{
			{Name: "1080p", Value: "1080"},
			{Name: "720p", Value: "720"},
			{Name: "480p", Value: "480"},
			{Name: "360p", Value: "360"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (media reference)"},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)"},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	height := vc.OptionalStringDefault("resolution", "720", inputs)

	inPath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the input video: " + err.Error())
	}
	ext := filepath.Ext(inPath)
	if ext == "" {
		ext = ".mp4"
	}
	outPath, err := flow.MediaScratchOutput(inPath, ext)
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.DefaultTimeout)
	defer cancel()

	// scale=-2:H keeps the aspect ratio and forces an even width (h264 needs it).
	stderr, err := vc.RunFFmpeg(ctx, "-y", "-i", inPath, "-vf", "scale=-2:"+height, "-c:a", "copy", outPath)
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
	var w, h int
	if p, e := vc.Probe(ctx, outPath); e == nil {
		w, h = p.Width, p.Height
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Scaled to %dx%d (%d bytes).", w, h, size),
		"video":       ref,
		"width":       w,
		"height":      h,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

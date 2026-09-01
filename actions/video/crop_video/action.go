// Package crop_video crops a rectangular region from a video, backed by ffmpeg's
// crop filter.
package crop_video

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
	Name         = "Crop Video"
	Description  = "Crop a rectangular region from a video by position and size"
	Website      = "https://www.flomation.co"
	Icon         = "object-group"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "x", Type: core.ConnectionTypeInteger, Label: "Left offset (px)", Value: 0},
	{Name: "y", Type: core.ConnectionTypeInteger, Label: "Top offset (px)", Value: 0},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)", Required: true},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px)", Required: true},
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
	x := vc.OptionalInt("x", 0, inputs)
	y := vc.OptionalInt("y", 0, inputs)
	w := vc.OptionalInt("width", 0, inputs)
	h := vc.OptionalInt("height", 0, inputs)
	if w <= 0 || h <= 0 {
		return vc.ErrResult("width and height must both be greater than zero")
	}
	if x < 0 || y < 0 {
		return vc.ErrResult("x and y offsets cannot be negative")
	}

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

	filter := fmt.Sprintf("crop=%d:%d:%d:%d", w, h, x, y)
	stderr, err := vc.RunFFmpeg(ctx, "-y", "-i", inPath, "-vf", filter, "-c:a", "copy", outPath)
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
		"tool_result": fmt.Sprintf("Cropped to %dx%d at (%d,%d) — %d bytes.", w, h, x, y, size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

// Package video_watermark overlays an image onto a video, backed by ffmpeg's
// overlay filter.
package video_watermark

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
	Name         = "Watermark Video"
	Description  = "Overlay an image (logo/watermark) onto a video at a chosen position"
	Website      = "https://www.flomation.co"
	Icon         = "layer-group"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "overlay", Type: core.ConnectionTypeString, Label: "Overlay image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "position", Type: core.ConnectionTypeString, Label: "Position", Value: "SouthEast",
		Options: []core.ConnectionOption{
			{Name: "Centre", Value: "Center"},
			{Name: "Top left", Value: "NorthWest"},
			{Name: "Top right", Value: "NorthEast"},
			{Name: "Bottom left", Value: "SouthWest"},
			{Name: "Bottom right", Value: "SouthEast"},
		},
	},
	{Name: "margin", Type: core.ConnectionTypeInteger, Label: "Margin (px)", Value: 10},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// overlayXY maps a gravity + margin to the ffmpeg overlay x:y expression, in terms
// of main_w/main_h (video) and overlay_w/overlay_h (image).
func overlayXY(position string, m int) string {
	switch position {
	case "Center":
		return "(main_w-overlay_w)/2:(main_h-overlay_h)/2"
	case "NorthWest":
		return fmt.Sprintf("%d:%d", m, m)
	case "NorthEast":
		return fmt.Sprintf("main_w-overlay_w-%d:%d", m, m)
	case "SouthWest":
		return fmt.Sprintf("%d:main_h-overlay_h-%d", m, m)
	default: // SouthEast
		return fmt.Sprintf("main_w-overlay_w-%d:main_h-overlay_h-%d", m, m)
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	overlay, err := vc.RequiredString("overlay", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	position := vc.OptionalStringDefault("position", "SouthEast", inputs)
	margin := vc.OptionalInt("margin", 10, inputs)
	if margin < 0 {
		margin = 0
	}

	videoPath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the input video: " + err.Error())
	}
	overlayPath, _, err := flow.ResolveToLocalFile(overlay)
	if err != nil {
		return vc.ErrResult("could not read the overlay image: " + err.Error())
	}
	ext := filepath.Ext(videoPath)
	if ext == "" {
		ext = ".mp4"
	}
	outPath, err := flow.MediaScratchOutput(videoPath, ext)
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.MaxTimeout)
	defer cancel()

	filter := "[0:v][1:v]overlay=" + overlayXY(position, margin)
	stderr, err := vc.RunFFmpeg(ctx, "-y", "-i", videoPath, "-i", overlayPath,
		"-filter_complex", filter, "-c:a", "copy", outPath)
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
		"tool_result": fmt.Sprintf("Watermarked the video at %s (%d bytes).", position, size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

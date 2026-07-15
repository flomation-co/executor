// Package overlay composites an animated (or static) graphic onto a base video —
// picture-in-picture / animated lower-thirds. ffmpeg overlay-backed.
package overlay

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
	Name         = "Overlay Video / Graphic"
	Description  = "Composite an animated or static graphic onto a video (picture-in-picture)"
	Website      = "https://www.flomation.co"
	Icon         = "layer-group"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Base video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "overlay", Type: core.ConnectionTypeString, Label: "Overlay graphic — video/GIF/image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "position", Type: core.ConnectionTypeString, Label: "Position", Value: "SouthEast",
		Options: []core.ConnectionOption{
			{Name: "Centre", Value: "Center"},
			{Name: "Top", Value: "Top"},
			{Name: "Bottom", Value: "Bottom"},
			{Name: "Top left", Value: "NorthWest"},
			{Name: "Top right", Value: "NorthEast"},
			{Name: "Bottom left", Value: "SouthWest"},
			{Name: "Bottom right", Value: "SouthEast"},
		},
	},
	{Name: "margin", Type: core.ConnectionTypeInteger, Label: "Margin (px)", Value: 20},
	{Name: "overlay_width", Type: core.ConnectionTypeInteger, Label: "Overlay width (px, 0 = original)", Value: 0},
	{Name: "loop", Type: core.ConnectionTypeBoolean, Label: "Loop the overlay to the video length", Value: true},
	{Name: "start_seconds", Type: core.ConnectionTypeString, Label: "Show from (seconds, blank = start)", Placeholder: "0"},
	{Name: "duration_seconds", Type: core.ConnectionTypeString, Label: "Show for (seconds, blank = whole)", Placeholder: "5"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// overlayXY maps a position + margin to the overlay x:y expression (main_w/main_h
// = base video, overlay_w/overlay_h = the graphic).
func overlayXY(position string, m int) string {
	switch position {
	case "Center":
		return "(main_w-overlay_w)/2:(main_h-overlay_h)/2"
	case "Top":
		return fmt.Sprintf("(main_w-overlay_w)/2:%d", m)
	case "Bottom":
		return fmt.Sprintf("(main_w-overlay_w)/2:main_h-overlay_h-%d", m)
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
	margin := vc.OptionalInt("margin", 20, inputs)
	if margin < 0 {
		margin = 0
	}
	overlayWidth := vc.OptionalInt("overlay_width", 0, inputs)
	loop := vc.OptionalBool("loop", true, inputs)
	start := vc.OptionalString("start_seconds", inputs)
	duration := vc.OptionalString("duration_seconds", inputs)

	basePath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the base video: " + err.Error())
	}
	overlayPath, _, err := flow.ResolveToLocalFile(overlay)
	if err != nil {
		return vc.ErrResult("could not read the overlay graphic: " + err.Error())
	}
	ext := filepath.Ext(basePath)
	if ext == "" {
		ext = ".mp4"
	}
	outPath, err := flow.MediaScratchFile(ext)
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.MaxTimeout)
	defer cancel()

	// Optionally scale the overlay first, then composite. enable=... times it.
	ovLabel := "[1:v]"
	filter := ""
	if overlayWidth > 0 {
		filter += fmt.Sprintf("[1:v]scale=%d:-1[ov];", overlayWidth)
		ovLabel = "[ov]"
	}
	ov := "overlay=" + overlayXY(position, margin)
	if start != "" || duration != "" {
		s := start
		if s == "" {
			s = "0"
		}
		if duration != "" {
			ov += fmt.Sprintf(":enable='between(t,%s,%s+%s)'", s, s, duration)
		} else {
			ov += fmt.Sprintf(":enable='gte(t,%s)'", s)
		}
	}
	if loop {
		// The looped overlay is an INFINITE stream; shortest=1 makes the overlay
		// end when the (finite) base video ends, so the output isn't infinite.
		ov += ":shortest=1"
	}
	filter += "[0:v]" + ovLabel + ov

	args := []string{"-y", "-i", basePath}
	if loop {
		args = append(args, "-stream_loop", "-1") // loop the overlay to the base length
	}
	args = append(args, "-i", overlayPath, "-filter_complex", filter, "-c:a", "copy", outPath)

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
		"tool_result": fmt.Sprintf("Overlaid the graphic at %s (%d bytes).", position, size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

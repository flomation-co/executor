// Package rotate_video rotates and/or flips a video, backed by ffmpeg's transpose/
// hflip/vflip filters.
package rotate_video

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Rotate / Flip Video"
	Description  = "Rotate a video by 90/180/270° and/or flip it horizontally or vertically"
	Website      = "https://www.flomation.co"
	Icon         = "rotate-right"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "angle", Type: core.ConnectionTypeString, Label: "Rotate", Value: "0",
		Options: []core.ConnectionOption{
			{Name: "None", Value: "0"},
			{Name: "90° clockwise", Value: "90"},
			{Name: "180°", Value: "180"},
			{Name: "270° clockwise", Value: "270"},
		},
	},
	{
		Name: "flip", Type: core.ConnectionTypeString, Label: "Flip", Value: "none",
		Options: []core.ConnectionOption{
			{Name: "None", Value: "none"},
			{Name: "Horizontal", Value: "horizontal"},
			{Name: "Vertical", Value: "vertical"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// filterChain builds the -vf value from the rotate + flip choices.
func filterChain(angle, flip string) string {
	var parts []string
	switch angle {
	case "90":
		parts = append(parts, "transpose=1")
	case "180":
		parts = append(parts, "transpose=2", "transpose=2")
	case "270":
		parts = append(parts, "transpose=2")
	}
	switch flip {
	case "horizontal":
		parts = append(parts, "hflip")
	case "vertical":
		parts = append(parts, "vflip")
	}
	return strings.Join(parts, ",")
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	angle := vc.OptionalStringDefault("angle", "0", inputs)
	flip := vc.OptionalStringDefault("flip", "none", inputs)

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

	chain := filterChain(angle, flip)
	args := []string{"-y", "-i", inPath}
	if chain != "" {
		args = append(args, "-vf", chain)
	}
	args = append(args, "-c:a", "copy", outPath)

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
		"tool_result": fmt.Sprintf("Rotated %s°, flip=%s (%d bytes).", angle, flip, size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

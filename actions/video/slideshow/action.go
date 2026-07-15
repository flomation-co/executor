// Package slideshow builds a video from a list of images, each shown for a fixed
// duration. ffmpeg-backed (concat demuxer + scale/pad to a common frame).
package slideshow

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Slideshow from Images"
	Description  = "Build a video from a list of images, each shown for a set number of seconds"
	Website      = "https://www.flomation.co"
	Icon         = "image+play"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "images", Type: core.ConnectionTypeText, Label: "Image references (one per line, in order)", Placeholder: "flo:file:…\nflo:blob:…", Required: true},
	{Name: "seconds_per_image", Type: core.ConnectionTypeString, Label: "Seconds per image", Value: "3", Placeholder: "3"},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)", Value: 1280},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (media reference)"},
	{Name: "image_count", Type: core.ConnectionTypeInteger, Label: "Images used"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

var listSplit = regexp.MustCompile(`[\r\n,]+`)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	raw, err := vc.RequiredString("images", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	seconds := vc.OptionalStringDefault("seconds_per_image", "3", inputs)
	width := vc.OptionalInt("width", 1280, inputs)
	if width < 2 {
		width = 1280
	}
	if width%2 != 0 {
		width++ // even width required by h264
	}
	height := (width * 9 / 16)
	if height%2 != 0 {
		height++
	}

	var paths []string
	for _, part := range listSplit.Split(raw, -1) {
		if p := strings.TrimSpace(part); p != "" {
			local, _, e := flow.ResolveToLocalFile(p)
			if e != nil {
				return vc.ErrResult("could not read an image: " + e.Error())
			}
			paths = append(paths, local)
		}
	}
	if len(paths) == 0 {
		return vc.ErrResult("provide at least one image reference")
	}

	// concat demuxer list with per-image durations. The last entry is repeated
	// without a duration so the final image is actually shown.
	var b strings.Builder
	for _, p := range paths {
		esc := strings.ReplaceAll(p, "'", `'\''`)
		b.WriteString("file '" + esc + "'\nduration " + seconds + "\n")
	}
	esc := strings.ReplaceAll(paths[len(paths)-1], "'", `'\''`)
	b.WriteString("file '" + esc + "'\n")

	listPath, err := flow.MediaScratchFile("txt")
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	if err := os.WriteFile(listPath, []byte(b.String()), 0o600); err != nil {
		return vc.ErrResult(err.Error())
	}
	outPath, err := flow.MediaScratchFile("mp4")
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.MaxTimeout)
	defer cancel()

	// Scale each image to fit the frame, pad to a uniform WxH, force 25fps.
	filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=25,format=yuv420p",
		width, height, width, height)
	stderr, err := vc.RunFFmpeg(ctx, "-y", "-f", "concat", "-safe", "0", "-i", listPath, "-vf", filter, outPath)
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
		"tool_result": fmt.Sprintf("Built a slideshow from %d image(s), %ss each (%d bytes).", len(paths), seconds, size),
		"video":       ref,
		"image_count": len(paths),
		"size_bytes":  size,
		"success":     true,
	}, nil
}

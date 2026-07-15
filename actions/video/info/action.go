// Package info reads a video's technical metadata via ffprobe. Read-only.
package info

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
	Name         = "Get Video Info"
	Description  = "Read a video's duration, resolution, codecs and bitrate (ffprobe)"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "duration_seconds", Type: core.ConnectionTypeString, Label: "Duration (seconds)"},
	{Name: "format", Type: core.ConnectionTypeString, Label: "Container format"},
	{Name: "video_codec", Type: core.ConnectionTypeString, Label: "Video codec"},
	{Name: "audio_codec", Type: core.ConnectionTypeString, Label: "Audio codec"},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)"},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px)"},
	{Name: "bitrate", Type: core.ConnectionTypeInteger, Label: "Bitrate (bps)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	inPath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the input video: " + err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.DefaultTimeout)
	defer cancel()

	p, err := vc.Probe(ctx, inPath)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	var size int64
	if fi, e := os.Stat(inPath); e == nil {
		size = fi.Size()
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("%s, %.1fs, %dx%d, video=%s audio=%s.", p.Format, p.DurationSeconds, p.Width, p.Height, p.VideoCodec, p.AudioCodec),
		"duration_seconds": fmt.Sprintf("%.3f", p.DurationSeconds),
		"format":           p.Format,
		"video_codec":      p.VideoCodec,
		"audio_codec":      p.AudioCodec,
		"width":            p.Width,
		"height":           p.Height,
		"bitrate":          p.BitRate,
		"size_bytes":       size,
		"success":          true,
	}, nil
}

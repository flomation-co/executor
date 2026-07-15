// Package transcode re-encodes a video to a chosen format + quality, backed by ffmpeg.
package transcode

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
	Name         = "Transcode Video"
	Description  = "Re-encode a video to a chosen format (codec + container) and quality"
	Website      = "https://www.flomation.co"
	Icon         = "gears"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		// Container + codec are one dropdown so only valid combinations are offered.
		Name: "format", Type: core.ConnectionTypeString, Label: "Format", Value: "mp4-h264",
		Options: []core.ConnectionOption{
			{Name: "MP4 (H.264)", Value: "mp4-h264"},
			{Name: "MP4 (H.265/HEVC)", Value: "mp4-h265"},
			{Name: "WebM (VP9)", Value: "webm-vp9"},
			{Name: "MKV (H.265)", Value: "mkv-h265"},
		},
	},
	{
		Name: "quality", Type: core.ConnectionTypeString, Label: "Quality", Value: "medium",
		Options: []core.ConnectionOption{
			{Name: "High (larger file)", Value: "high"},
			{Name: "Medium", Value: "medium"},
			{Name: "Low (smaller file)", Value: "low"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (media reference)"},
	{Name: "format", Type: core.ConnectionTypeString, Label: "Format"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type formatSpec struct {
	container string
	vcodec    string
	acodec    string
}

var formats = map[string]formatSpec{
	"mp4-h264": {"mp4", "libx264", "aac"},
	"mp4-h265": {"mp4", "libx265", "aac"},
	"webm-vp9": {"webm", "libvpx-vp9", "libopus"},
	"mkv-h265": {"mkv", "libx265", "aac"},
}

var crf = map[string]string{"high": "18", "medium": "23", "low": "28"}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	format := vc.OptionalStringDefault("format", "mp4-h264", inputs)
	spec, ok := formats[format]
	if !ok {
		return vc.ErrResult("unknown format: " + format)
	}
	q := crf[vc.OptionalStringDefault("quality", "medium", inputs)]
	if q == "" {
		q = "23"
	}

	inPath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the input video: " + err.Error())
	}
	outPath, err := flow.MediaScratchFile(spec.container)
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.MaxTimeout)
	defer cancel()

	stderr, err := vc.RunFFmpeg(ctx, "-y", "-i", inPath,
		"-c:v", spec.vcodec, "-crf", q, "-c:a", spec.acodec, outPath)
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
		"tool_result": fmt.Sprintf("Transcoded to %s (%d bytes).", format, size),
		"video":       ref,
		"format":      format,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

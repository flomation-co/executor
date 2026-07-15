// Package extract_audio pulls the audio track out of a video file and returns it
// as a workspace file reference (flo:file:). ffmpeg-backed.
package extract_audio

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
	Name         = "Extract Audio"
	Description  = "Extract the audio track from a video as mp3/aac/wav/ogg"
	Website      = "https://www.flomation.co"
	Icon         = "microphone"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "video",
		Type:        core.ConnectionTypeString,
		Label:       "Video (file/blob reference)",
		Placeholder: "flo:file:… or flo:blob:…",
		Required:    true,
	},
	{
		Name:  "format",
		Type:  core.ConnectionTypeString,
		Label: "Audio format",
		Value: "mp3",
		Options: []core.ConnectionOption{
			{Name: "MP3", Value: "mp3"},
			{Name: "AAC", Value: "aac"},
			{Name: "WAV", Value: "wav"},
			{Name: "OGG", Value: "ogg"},
		},
	},
	{
		Name:  "bitrate",
		Type:  core.ConnectionTypeString,
		Label: "Bitrate",
		Value: "192k",
		Options: []core.ConnectionOption{
			{Name: "128 kbps", Value: "128k"},
			{Name: "192 kbps", Value: "192k"},
			{Name: "256 kbps", Value: "256k"},
			{Name: "320 kbps", Value: "320k"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "audio", Type: core.ConnectionTypeString, Label: "Audio (media reference)"},
	{Name: "format", Type: core.ConnectionTypeString, Label: "Audio format"},
	{Name: "duration_seconds", Type: core.ConnectionTypeString, Label: "Duration (seconds)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// audioCodec maps a container/format choice to the ffmpeg encoder.
func audioCodec(format string) string {
	switch format {
	case "aac":
		return "aac"
	case "wav":
		return "pcm_s16le"
	case "ogg":
		return "libvorbis"
	default:
		return "libmp3lame"
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	format := vc.OptionalStringDefault("format", "mp3", inputs)
	bitrate := vc.OptionalStringDefault("bitrate", "192k", inputs)

	inPath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the input video: " + err.Error())
	}

	outPath, err := flow.MediaScratchFile(format)
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.DefaultTimeout)
	defer cancel()

	// -vn drops the video; -c:a picks the encoder; -y overwrites the scratch file.
	stderr, err := vc.RunFFmpeg(ctx, "-y", "-i", inPath, "-vn",
		"-c:a", audioCodec(format), "-b:a", bitrate, outPath)
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
	var duration float64
	if p, e := vc.Probe(ctx, outPath); e == nil {
		duration = p.DurationSeconds
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Extracted %s audio (%.1fs, %d bytes).", format, duration, size),
		"audio":            ref,
		"format":           format,
		"duration_seconds": fmt.Sprintf("%.3f", duration),
		"size_bytes":       size,
		"success":          true,
	}, nil
}

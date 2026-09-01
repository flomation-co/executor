// Package convert_audio re-encodes an audio file to a different format/bitrate,
// backed by ffmpeg.
package convert_audio

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
	Name         = "Convert Audio"
	Description  = "Convert an audio file to mp3/aac/wav/ogg/flac at a chosen bitrate"
	Website      = "https://www.flomation.co"
	Icon         = "arrow-right-arrow-left"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "audio", Type: core.ConnectionTypeString, Label: "Audio (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "format", Type: core.ConnectionTypeString, Label: "Format", Value: "mp3",
		Options: []core.ConnectionOption{
			{Name: "MP3", Value: "mp3"},
			{Name: "AAC", Value: "aac"},
			{Name: "WAV (lossless)", Value: "wav"},
			{Name: "OGG", Value: "ogg"},
			{Name: "FLAC (lossless)", Value: "flac"},
		},
	},
	{
		Name: "bitrate", Type: core.ConnectionTypeString, Label: "Bitrate (lossy formats)", Value: "192k",
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
	{Name: "format", Type: core.ConnectionTypeString, Label: "Format"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func codecAndLossless(format string) (codec string, lossless bool) {
	switch format {
	case "aac":
		return "aac", false
	case "wav":
		return "pcm_s16le", true
	case "ogg":
		return "libvorbis", false
	case "flac":
		return "flac", true
	default:
		return "libmp3lame", false
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	audio, err := vc.RequiredString("audio", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	format := vc.OptionalStringDefault("format", "mp3", inputs)
	bitrate := vc.OptionalStringDefault("bitrate", "192k", inputs)

	inPath, _, err := flow.ResolveToLocalFile(audio)
	if err != nil {
		return vc.ErrResult("could not read the input audio: " + err.Error())
	}
	outPath, err := flow.MediaScratchOutput(inPath, format)
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.DefaultTimeout)
	defer cancel()

	codec, lossless := codecAndLossless(format)
	args := []string{"-y", "-i", inPath, "-vn", "-c:a", codec}
	if !lossless {
		args = append(args, "-b:a", bitrate)
	}
	args = append(args, outPath)

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
		"tool_result": fmt.Sprintf("Converted to %s (%d bytes).", format, size),
		"audio":       ref,
		"format":      format,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

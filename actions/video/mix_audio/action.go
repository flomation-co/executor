// Package mix_audio mixes two audio tracks into one (e.g. background music under a
// voiceover), backed by ffmpeg's amix filter.
package mix_audio

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
	Name         = "Mix Audio"
	Description  = "Mix two audio tracks into one (e.g. background music under a voiceover)"
	Website      = "https://www.flomation.co"
	Icon         = "microphone"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "audio_a", Type: core.ConnectionTypeString, Label: "First audio (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "audio_b", Type: core.ConnectionTypeString, Label: "Second audio (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "duration", Type: core.ConnectionTypeString, Label: "Result length", Value: "longest",
		Options: []core.ConnectionOption{
			{Name: "As long as the longest", Value: "longest"},
			{Name: "As long as the shortest", Value: "shortest"},
			{Name: "As long as the first", Value: "first"},
		},
	},
	{
		Name: "format", Type: core.ConnectionTypeString, Label: "Format", Value: "mp3",
		Options: []core.ConnectionOption{
			{Name: "MP3", Value: "mp3"},
			{Name: "AAC", Value: "aac"},
			{Name: "WAV", Value: "wav"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "audio", Type: core.ConnectionTypeString, Label: "Audio (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func codec(format string) string {
	switch format {
	case "aac":
		return "aac"
	case "wav":
		return "pcm_s16le"
	default:
		return "libmp3lame"
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	a, err := vc.RequiredString("audio_a", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	b, err := vc.RequiredString("audio_b", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	duration := vc.OptionalStringDefault("duration", "longest", inputs)
	format := vc.OptionalStringDefault("format", "mp3", inputs)

	aPath, _, err := flow.ResolveToLocalFile(a)
	if err != nil {
		return vc.ErrResult("could not read the first audio: " + err.Error())
	}
	bPath, _, err := flow.ResolveToLocalFile(b)
	if err != nil {
		return vc.ErrResult("could not read the second audio: " + err.Error())
	}
	outPath, err := flow.MediaScratchOutput(aPath, format)
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.DefaultTimeout)
	defer cancel()

	filter := fmt.Sprintf("amix=inputs=2:duration=%s:dropout_transition=0", duration)
	stderr, err := vc.RunFFmpeg(ctx, "-y", "-i", aPath, "-i", bPath,
		"-filter_complex", filter, "-c:a", codec(format), outPath)
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
		"tool_result": fmt.Sprintf("Mixed two tracks (%d bytes).", size),
		"audio":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

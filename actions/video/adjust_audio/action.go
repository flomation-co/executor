// Package adjust_audio changes the loudness of an audio or video's audio track:
// volume, loudness-normalise, or fade in/out. ffmpeg-backed.
package adjust_audio

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
	Name         = "Adjust Audio"
	Description  = "Change volume, loudness-normalise, or fade an audio or video's audio in/out"
	Website      = "https://www.flomation.co"
	Icon         = "microphone"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "media", Type: core.ConnectionTypeString, Label: "Audio or video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "operation", Type: core.ConnectionTypeString, Label: "Operation", Value: "normalize",
		Options: []core.ConnectionOption{
			{Name: "Normalise loudness", Value: "normalize"},
			{Name: "Change volume (dB)", Value: "volume"},
			{Name: "Fade in", Value: "fade_in"},
			{Name: "Fade out", Value: "fade_out"},
		},
	},
	{Name: "amount", Type: core.ConnectionTypeString, Label: "Amount (dB for volume, seconds for fades)", Value: "3", Placeholder: "3"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "media", Type: core.ConnectionTypeString, Label: "Media (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	media, err := vc.RequiredString("media", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	operation := vc.OptionalStringDefault("operation", "normalize", inputs)
	amount := vc.OptionalFloat("amount", 3, inputs)

	inPath, _, err := flow.ResolveToLocalFile(media)
	if err != nil {
		return vc.ErrResult("could not read the input: " + err.Error())
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

	probe, _ := vc.Probe(ctx, inPath) // best-effort: duration + whether there's video

	var af string
	switch operation {
	case "volume":
		af = fmt.Sprintf("volume=%.2fdB", amount)
	case "fade_in":
		af = fmt.Sprintf("afade=t=in:st=0:d=%.2f", amount)
	case "fade_out":
		if probe == nil || probe.DurationSeconds <= 0 {
			return vc.ErrResult("fade out needs the media duration, which could not be read")
		}
		st := probe.DurationSeconds - amount
		if st < 0 {
			st = 0
		}
		af = fmt.Sprintf("afade=t=out:st=%.2f:d=%.2f", st, amount)
	default: // normalize
		af = "loudnorm"
	}

	args := []string{"-y", "-i", inPath}
	// Keep the video stream untouched when the input is a video.
	if probe != nil && probe.VideoCodec != "" {
		args = append(args, "-c:v", "copy")
	}
	args = append(args, "-af", af, outPath)

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
		"tool_result": fmt.Sprintf("Applied %s to the audio (%d bytes).", operation, size),
		"media":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

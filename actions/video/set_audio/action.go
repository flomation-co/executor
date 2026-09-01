// Package set_audio replaces a video's audio track with a supplied audio file,
// backed by ffmpeg.
package set_audio

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
	Name         = "Set Video Audio"
	Description  = "Replace a video's audio track with a supplied audio file"
	Website      = "https://www.flomation.co"
	Icon         = "microphone+plus"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "audio", Type: core.ConnectionTypeString, Label: "Audio (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "shortest", Type: core.ConnectionTypeBoolean, Label: "Trim to the shorter track", Value: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	audio, err := vc.RequiredString("audio", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	shortest := vc.OptionalBool("shortest", true, inputs)

	videoPath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the input video: " + err.Error())
	}
	audioPath, _, err := flow.ResolveToLocalFile(audio)
	if err != nil {
		return vc.ErrResult("could not read the input audio: " + err.Error())
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

	// Take the video stream from input 0 and the audio from input 1; copy the
	// video, re-encode the audio to AAC.
	args := []string{"-y", "-i", videoPath, "-i", audioPath,
		"-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "aac"}
	if shortest {
		args = append(args, "-shortest")
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
		"tool_result": fmt.Sprintf("Set the video's audio track (%d bytes).", size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

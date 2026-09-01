// Package speed changes a video's playback speed, backed by ffmpeg (setpts for the
// video, a chained atempo for the audio).
package speed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Change Video Speed"
	Description  = "Speed up or slow down a video (audio pitch is preserved)"
	Website      = "https://www.flomation.co"
	Icon         = "gauge"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "factor", Type: core.ConnectionTypeString, Label: "Speed", Value: "2",
		Options: []core.ConnectionOption{
			{Name: "0.5× (half speed)", Value: "0.5"},
			{Name: "1× (unchanged)", Value: "1"},
			{Name: "1.5×", Value: "1.5"},
			{Name: "2×", Value: "2"},
			{Name: "4×", Value: "4"},
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

// buildAtempo decomposes a speed factor into a chain of atempo filters, each
// within ffmpeg's supported 0.5–2.0 range (their product equals the factor).
func buildAtempo(factor float64) string {
	var parts []string
	f := factor
	for f > 2.0 {
		parts = append(parts, "atempo=2.0")
		f /= 2.0
	}
	for f < 0.5 {
		parts = append(parts, "atempo=0.5")
		f *= 2.0
	}
	parts = append(parts, "atempo="+strconv.FormatFloat(f, 'f', -1, 64))
	return strings.Join(parts, ",")
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	factor := vc.OptionalFloat("factor", 2, inputs)
	if factor <= 0 {
		return vc.ErrResult("speed factor must be greater than zero")
	}

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

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.MaxTimeout)
	defer cancel()

	probe, _ := vc.Probe(ctx, inPath)
	hasAudio := probe != nil && probe.AudioCodec != ""

	// setpts scales presentation timestamps: PTS/factor speeds up, *factor slows.
	vpts := "setpts=" + strconv.FormatFloat(1.0/factor, 'f', -1, 64) + "*PTS"

	var args []string
	if hasAudio {
		fc := fmt.Sprintf("[0:v]%s[v];[0:a]%s[a]", vpts, buildAtempo(factor))
		args = []string{"-y", "-i", inPath, "-filter_complex", fc, "-map", "[v]", "-map", "[a]", outPath}
	} else {
		args = []string{"-y", "-i", inPath, "-vf", vpts, "-an", outPath}
	}

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
		"tool_result": fmt.Sprintf("Changed speed to %g× (%d bytes).", factor, size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

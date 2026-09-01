// Package concat joins several videos end to end, backed by ffmpeg's concat demuxer.
package concat

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
	Name         = "Concatenate Videos"
	Description  = "Join several videos end to end into one (given as a list of references)"
	Website      = "https://www.flomation.co"
	Icon         = "link"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "videos", Type: core.ConnectionTypeText, Label: "Video references (one per line, in order)", Placeholder: "flo:file:…\nflo:blob:…", Required: true},
	{Name: "reencode", Type: core.ConnectionTypeBoolean, Label: "Re-encode (use if the clips differ)", Value: false},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (media reference)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Clips joined"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

var listSplit = regexp.MustCompile(`[\r\n,]+`)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	raw, err := vc.RequiredString("videos", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	reencode := vc.OptionalBool("reencode", false, inputs)

	// Split on newlines/commas; keep order, drop blanks.
	var refs []string
	for _, part := range listSplit.Split(raw, -1) {
		if p := strings.TrimSpace(part); p != "" {
			refs = append(refs, p)
		}
	}
	if len(refs) < 2 {
		return vc.ErrResult("provide at least two video references to concatenate")
	}

	// Resolve each reference to a local file and build the concat list.
	var listBuilder strings.Builder
	var paths []string
	for _, r := range refs {
		p, _, e := flow.ResolveToLocalFile(r)
		if e != nil {
			return vc.ErrResult("could not read a video: " + e.Error())
		}
		paths = append(paths, p)
		// ffmpeg concat list format: file '<path>' — escape any single quotes.
		esc := strings.ReplaceAll(p, "'", `'\''`)
		listBuilder.WriteString("file '" + esc + "'\n")
	}

	listPath, err := flow.MediaScratchFile("txt")
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	if err := os.WriteFile(listPath, []byte(listBuilder.String()), 0o600); err != nil {
		return vc.ErrResult(err.Error())
	}
	outPath, err := flow.MediaScratchOutputFor(paths, "concat", "mp4")
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.MaxTimeout)
	defer cancel()

	// -safe 0 permits absolute paths in the list; stream-copy unless re-encoding.
	args := []string{"-y", "-f", "concat", "-safe", "0", "-i", listPath}
	if !reencode {
		args = append(args, "-c", "copy")
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
		"tool_result": fmt.Sprintf("Joined %d clips (%d bytes).", len(refs), size),
		"video":       ref,
		"count":       len(refs),
		"size_bytes":  size,
		"success":     true,
	}, nil
}

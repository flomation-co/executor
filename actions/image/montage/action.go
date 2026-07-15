// Package montage arranges a list of images into a grid (contact sheet / collage),
// backed by ImageMagick's montage sub-command.
package montage

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	core "flomation.app/automate/executor"
	ic "flomation.app/automate/executor/actions/image"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Montage / Grid"
	Description  = "Arrange several images into a grid (contact sheet or collage)"
	Website      = "https://www.flomation.co"
	Icon         = "grip"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "images", Type: core.ConnectionTypeText, Label: "Image references (one per line, in order)", Placeholder: "flo:file:…\nflo:blob:…", Required: true},
	{Name: "columns", Type: core.ConnectionTypeInteger, Label: "Columns", Value: 3},
	{Name: "tile_size", Type: core.ConnectionTypeInteger, Label: "Cell size (px)", Value: 200},
	{Name: "spacing", Type: core.ConnectionTypeInteger, Label: "Spacing (px)", Value: 5},
	{Name: "background", Type: core.ConnectionTypeString, Label: "Background colour", Value: "white"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (media reference)"},
	{Name: "image_count", Type: core.ConnectionTypeInteger, Label: "Images used"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

var listSplit = regexp.MustCompile(`[\r\n,]+`)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	raw, err := ic.RequiredString("images", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	cols := ic.OptionalInt("columns", 3, inputs)
	if cols < 1 {
		cols = 1
	}
	tile := ic.OptionalInt("tile_size", 200, inputs)
	if tile < 1 {
		tile = 200
	}
	spacing := ic.OptionalInt("spacing", 5, inputs)
	if spacing < 0 {
		spacing = 0
	}
	background := ic.OptionalStringDefault("background", "white", inputs)

	var paths []string
	for _, part := range listSplit.Split(raw, -1) {
		if p := strings.TrimSpace(part); p != "" {
			local, _, e := flow.ResolveToLocalFile(p)
			if e != nil {
				return ic.ErrResult("could not read an image: " + e.Error())
			}
			paths = append(paths, local)
		}
	}
	if len(paths) == 0 {
		return ic.ErrResult("provide at least one image reference")
	}

	outPath, err := flow.MediaScratchFile("png")
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	// montage <images…> -tile <cols>x -geometry <tile>x<tile>+<pad>+<pad> …
	// (the montage sub-command must lead, so RunMagickSub places limits after it).
	var args []string
	if font, ok := ic.FindFont(); ok {
		args = append(args, "-font", font) // avoids the label-font warning
	}
	args = append(args, paths...)
	args = append(args,
		"-tile", fmt.Sprintf("%dx", cols),
		"-geometry", fmt.Sprintf("%dx%d+%d+%d", tile, tile, spacing, spacing),
		"-background", background,
		outPath)

	stderr, runErr := ic.RunMagickSub(ctx, "montage", args...)
	// montage exits non-zero when it can't find a *label* font, yet still produces
	// a valid tiled image (we use no labels). Treat a non-empty output as success.
	fi, statErr := os.Stat(outPath)
	if statErr != nil || fi.Size() == 0 {
		return ic.ErrResult(fmt.Sprintf("magick montage failed: %v: %s", runErr, ic.Tail(stderr, 400)))
	}

	ref, err := flow.EmitMediaFile(outPath)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	var size int64
	if fi, e := os.Stat(outPath); e == nil {
		size = fi.Size()
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Built a %d-image montage, %d column(s) (%d bytes).", len(paths), cols, size),
		"image":       ref,
		"image_count": len(paths),
		"size_bytes":  size,
		"success":     true,
	}, nil
}

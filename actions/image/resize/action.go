// Package resize scales an image, backed by ImageMagick.
package resize

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	core "flomation.app/automate/executor"
	ic "flomation.app/automate/executor/actions/image"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Resize Image"
	Description  = "Resize an image to a width and/or height with a chosen fit mode"
	Website      = "https://www.flomation.co"
	Icon         = "expand"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px, 0 = auto)", Value: 0},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px, 0 = auto)", Value: 0},
	{
		Name: "fit", Type: core.ConnectionTypeString, Label: "Fit mode", Value: "fit",
		Options: []core.ConnectionOption{
			{Name: "Fit within (keep aspect)", Value: "fit"},
			{Name: "Fill & crop (cover)", Value: "fill"},
			{Name: "Stretch (ignore aspect)", Value: "stretch"},
		},
	},
	{Name: "allow_upscale", Type: core.ConnectionTypeBoolean, Label: "Allow enlarging", Value: false},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file reference)"},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)"},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func geometry(w, h int) string {
	switch {
	case w > 0 && h > 0:
		return fmt.Sprintf("%dx%d", w, h)
	case w > 0:
		return fmt.Sprintf("%d", w)
	default:
		return fmt.Sprintf("x%d", h)
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	image, err := ic.RequiredString("image", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	w := ic.OptionalInt("width", 0, inputs)
	h := ic.OptionalInt("height", 0, inputs)
	if w <= 0 && h <= 0 {
		return ic.ErrResult("provide a width and/or height to resize to")
	}
	fit := ic.OptionalStringDefault("fit", "fit", inputs)
	upscale := ic.OptionalBool("allow_upscale", false, inputs)

	inPath, _, err := flow.ResolveToLocalFile(image)
	if err != nil {
		return ic.ErrResult("could not read the input image: " + err.Error())
	}
	ext := filepath.Ext(inPath)
	if ext == "" {
		ext = ".png"
	}
	outPath, err := flow.MediaScratchFile(ext)
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	geo := geometry(w, h)
	args := []string{inPath}
	switch fit {
	case "stretch":
		args = append(args, "-resize", geo+"!")
	case "fill":
		args = append(args, "-resize", geo+"^", "-gravity", "center", "-extent", geo)
	default: // fit
		if !upscale {
			geo += ">"
		}
		args = append(args, "-resize", geo)
	}
	args = append(args, outPath)

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	stderr, err := ic.RunMagick(ctx, args...)
	if err != nil {
		return ic.ErrResult(fmt.Sprintf("magick failed: %v: %s", err, ic.Tail(stderr, 400)))
	}

	ref, err := flow.EmitLocalFile(outPath)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	var size int64
	if fi, e := os.Stat(outPath); e == nil {
		size = fi.Size()
	}
	outW, outH := w, h
	if info, e := ic.Identify(ctx, outPath); e == nil {
		outW, outH = info.Width, info.Height
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Resized to %dx%d (%d bytes).", outW, outH, size),
		"image":       ref,
		"width":       outW,
		"height":      outH,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

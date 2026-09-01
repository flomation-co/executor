// Package crop cuts a rectangular region out of an image, backed by ImageMagick.
package crop

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
	Name         = "Crop Image"
	Description  = "Crop a rectangular region from an image by position and size"
	Website      = "https://www.flomation.co"
	Icon         = "object-group"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "x", Type: core.ConnectionTypeInteger, Label: "Left offset (px)", Value: 0},
	{Name: "y", Type: core.ConnectionTypeInteger, Label: "Top offset (px)", Value: 0},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)", Required: true},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (media reference)"},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)"},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	image, err := ic.RequiredString("image", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	x := ic.OptionalInt("x", 0, inputs)
	y := ic.OptionalInt("y", 0, inputs)
	w := ic.OptionalInt("width", 0, inputs)
	h := ic.OptionalInt("height", 0, inputs)
	if w <= 0 || h <= 0 {
		return ic.ErrResult("width and height must both be greater than zero")
	}
	if x < 0 || y < 0 {
		return ic.ErrResult("x and y offsets cannot be negative")
	}

	inPath, _, err := flow.ResolveToLocalFile(image)
	if err != nil {
		return ic.ErrResult("could not read the input image: " + err.Error())
	}
	ext := filepath.Ext(inPath)
	if ext == "" {
		ext = ".png"
	}
	outPath, err := flow.MediaScratchOutput(inPath, ext)
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	// +repage resets the virtual canvas so the crop geometry isn't retained.
	cropGeo := fmt.Sprintf("%dx%d+%d+%d", w, h, x, y)
	stderr, err := ic.RunMagick(ctx, inPath, "-crop", cropGeo, "+repage", outPath)
	if err != nil {
		return ic.ErrResult(fmt.Sprintf("magick failed: %v: %s", err, ic.Tail(stderr, 400)))
	}

	ref, err := flow.EmitMediaFile(outPath)
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
		"tool_result": fmt.Sprintf("Cropped to %dx%d at (%d,%d) — %d bytes.", outW, outH, x, y, size),
		"image":       ref,
		"width":       outW,
		"height":      outH,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

// Package create generates a new solid-colour image from nothing — a media
// "source" node. ImageMagick-backed.
package create

import (
	"context"
	"fmt"
	"os"

	core "flomation.app/automate/executor"
	ic "flomation.app/automate/executor/actions/image"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Image"
	Description  = "Create a new blank image of a given size and colour"
	Website      = "https://www.flomation.co"
	Icon         = "image+plus"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (px)", Value: 512},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (px)", Value: 512},
	{Name: "colour", Type: core.ConnectionTypeString, Label: "Colour", Value: "white", Placeholder: "white, #ff0000, transparent"},
	{
		Name: "format", Type: core.ConnectionTypeString, Label: "Format", Value: "png",
		Options: []core.ConnectionOption{
			{Name: "PNG", Value: "png"},
			{Name: "JPEG", Value: "jpg"},
		},
	},
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
	w := ic.OptionalInt("width", 512, inputs)
	h := ic.OptionalInt("height", 512, inputs)
	if w < 1 || h < 1 {
		return ic.ErrResult("width and height must be greater than zero")
	}
	if w > 10000 || h > 10000 {
		return ic.ErrResult("width and height must be 10000px or less")
	}
	colour := ic.OptionalStringDefault("colour", "white", inputs)
	format := ic.OptionalStringDefault("format", "png", inputs)

	outPath, err := flow.MediaScratchFileNamed("image." + format)
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	// -size WxH  xc:<colour>  → a solid canvas. colour is a single argv token.
	stderr, err := ic.RunMagick(ctx, "-size", fmt.Sprintf("%dx%d", w, h), "xc:"+colour, outPath)
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

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created a %dx%d %s image (%d bytes).", w, h, colour, size),
		"image":       ref,
		"width":       w,
		"height":      h,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

// Package round_corners rounds the corners of an image (transparent outside the
// radius), backed by ImageMagick. Output is always PNG so the transparency is kept.
package round_corners

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
	Name         = "Round Corners"
	Description  = "Round the corners of an image (transparent PNG output)"
	Website      = "https://www.flomation.co"
	Icon         = "image"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "radius", Type: core.ConnectionTypeInteger, Label: "Corner radius (px)", Value: 24},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	image, err := ic.RequiredString("image", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	r := ic.OptionalInt("radius", 24, inputs)
	if r < 1 {
		r = 1
	}

	inPath, _, err := flow.ResolveToLocalFile(image)
	if err != nil {
		return ic.ErrResult("could not read the input image: " + err.Error())
	}
	outPath, err := flow.MediaScratchOutput(inPath, "png") // needs alpha
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	// Standard rounded-corner recipe: build a rounded mask from one corner
	// (mirrored across both axes) and copy it into the alpha channel.
	draw := fmt.Sprintf("fill black polygon 0,0 0,%d %d,0 fill white circle %d,%d %d,0", r, r, r, r, r)
	stderr, err := ic.RunMagick(ctx, inPath,
		"(", "+clone", "-alpha", "extract",
		"-draw", draw,
		"(", "+clone", "-flip", ")", "-compose", "Multiply", "-composite",
		"(", "+clone", "-flop", ")", "-compose", "Multiply", "-composite",
		")", "-alpha", "off", "-compose", "CopyOpacity", "-composite", outPath)
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
		"tool_result": fmt.Sprintf("Rounded corners at %dpx radius (%d bytes).", r, size),
		"image":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

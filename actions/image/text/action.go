// Package text draws text onto an image, backed by ImageMagick.
package text

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
	Name         = "Add Text to Image"
	Description  = "Draw a text caption onto an image at a chosen position, size and colour"
	Website      = "https://www.flomation.co"
	Icon         = "pen"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Text", Placeholder: "Your caption", Required: true},
	{Name: "font_size", Type: core.ConnectionTypeInteger, Label: "Font size (px)", Value: 24},
	{Name: "colour", Type: core.ConnectionTypeString, Label: "Colour", Value: "white", Placeholder: "white, #ff0000, rgba(0,0,0,0.5)"},
	{
		Name: "gravity", Type: core.ConnectionTypeString, Label: "Position", Value: "SouthWest",
		Options: []core.ConnectionOption{
			{Name: "Centre", Value: "Center"},
			{Name: "Top left", Value: "NorthWest"},
			{Name: "Top right", Value: "NorthEast"},
			{Name: "Bottom left", Value: "SouthWest"},
			{Name: "Bottom right", Value: "SouthEast"},
		},
	},
	{Name: "x", Type: core.ConnectionTypeInteger, Label: "X offset (px)", Value: 10},
	{Name: "y", Type: core.ConnectionTypeInteger, Label: "Y offset (px)", Value: 10},
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
	caption, err := ic.RequiredString("text", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	fontSize := ic.OptionalInt("font_size", 24, inputs)
	if fontSize < 1 {
		fontSize = 1
	}
	colour := ic.OptionalStringDefault("colour", "white", inputs)
	gravity := ic.OptionalStringDefault("gravity", "SouthWest", inputs)
	x := ic.OptionalInt("x", 10, inputs)
	y := ic.OptionalInt("y", 10, inputs)

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

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	// The caption is passed as a single argv token (never a shell string), so
	// arbitrary text — including quotes/newlines — is safe.
	offset := fmt.Sprintf("+%d+%d", x, y)
	stderr, err := ic.RunMagick(ctx, inPath,
		"-gravity", gravity, "-pointsize", fmt.Sprintf("%d", fontSize), "-fill", colour,
		"-annotate", offset, caption, outPath)
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
		"tool_result": fmt.Sprintf("Added text at %s (%d bytes).", gravity, size),
		"image":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

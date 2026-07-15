// Package convert changes an image's file format, backed by ImageMagick.
package convert

import (
	"context"
	"fmt"
	"os"
	"strconv"

	core "flomation.app/automate/executor"
	ic "flomation.app/automate/executor/actions/image"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Convert Image Format"
	Description  = "Convert an image to PNG, JPEG, WebP, GIF, TIFF or AVIF"
	Website      = "https://www.flomation.co"
	Icon         = "arrow-right-arrow-left"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "format", Type: core.ConnectionTypeString, Label: "Target format", Value: "png",
		Options: []core.ConnectionOption{
			{Name: "PNG", Value: "png"},
			{Name: "JPEG", Value: "jpg"},
			{Name: "WebP", Value: "webp"},
			{Name: "GIF", Value: "gif"},
			{Name: "TIFF", Value: "tiff"},
			{Name: "AVIF", Value: "avif"},
		},
	},
	{Name: "quality", Type: core.ConnectionTypeInteger, Label: "Quality (1–100, lossy formats)", Value: 85},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file reference)"},
	{Name: "format", Type: core.ConnectionTypeString, Label: "Format"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	image, err := ic.RequiredString("image", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	format := ic.OptionalStringDefault("format", "png", inputs)
	quality := ic.OptionalInt("quality", 85, inputs)
	if quality < 1 {
		quality = 1
	} else if quality > 100 {
		quality = 100
	}

	inPath, _, err := flow.ResolveToLocalFile(image)
	if err != nil {
		return ic.ErrResult("could not read the input image: " + err.Error())
	}
	outPath, err := flow.MediaScratchFile(format)
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	stderr, err := ic.RunMagick(ctx, inPath, "-quality", strconv.Itoa(quality), outPath)
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

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Converted to %s (%d bytes).", format, size),
		"image":       ref,
		"format":      format,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

// Package optimize strips metadata and recompresses an image for a smaller file.
package optimize

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	core "flomation.app/automate/executor"
	ic "flomation.app/automate/executor/actions/image"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Optimize Image"
	Description  = "Strip metadata and recompress an image to reduce its file size"
	Website      = "https://www.flomation.co"
	Icon         = "compress"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "quality", Type: core.ConnectionTypeInteger, Label: "Quality (1–100, lossy formats)", Value: 82},
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
	quality := ic.OptionalInt("quality", 82, inputs)
	if quality < 1 {
		quality = 1
	} else if quality > 100 {
		quality = 100
	}

	inPath, _, err := flow.ResolveToLocalFile(image)
	if err != nil {
		return ic.ErrResult("could not read the input image: " + err.Error())
	}
	ext := filepath.Ext(inPath)
	if ext == "" {
		ext = ".jpg"
	}
	outPath, err := flow.MediaScratchFile(ext)
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	stderr, err := ic.RunMagick(ctx, inPath, "-strip", "-quality", strconv.Itoa(quality), outPath)
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
		"tool_result": fmt.Sprintf("Optimised the image (%d bytes).", size),
		"image":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

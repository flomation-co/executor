// Package autocrop trims a uniform surrounding border from an image.
package autocrop

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
	Name         = "Auto-crop Image"
	Description  = "Automatically trim a uniform (e.g. whitespace) border from an image"
	Website      = "https://www.flomation.co"
	Icon         = "object-group"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "fuzz", Type: core.ConnectionTypeInteger, Label: "Colour tolerance (%)", Value: 5},
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
	fuzz := ic.OptionalInt("fuzz", 5, inputs)
	if fuzz < 0 {
		fuzz = 0
	} else if fuzz > 100 {
		fuzz = 100
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

	stderr, err := ic.RunMagick(ctx, inPath, "-fuzz", fmt.Sprintf("%d%%", fuzz), "-trim", "+repage", outPath)
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
	w, h := 0, 0
	if info, e := ic.Identify(ctx, outPath); e == nil {
		w, h = info.Width, info.Height
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Trimmed to %dx%d (%d bytes).", w, h, size),
		"image":       ref,
		"width":       w,
		"height":      h,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

// Package rotate rotates and/or flips an image, backed by ImageMagick.
package rotate

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
	Name         = "Rotate / Flip Image"
	Description  = "Rotate an image by 90/180/270° and/or flip it horizontally or vertically"
	Website      = "https://www.flomation.co"
	Icon         = "rotate-right"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "angle", Type: core.ConnectionTypeString, Label: "Rotate", Value: "0",
		Options: []core.ConnectionOption{
			{Name: "None", Value: "0"},
			{Name: "90° clockwise", Value: "90"},
			{Name: "180°", Value: "180"},
			{Name: "270° clockwise", Value: "270"},
		},
	},
	{
		Name: "flip", Type: core.ConnectionTypeString, Label: "Flip", Value: "none",
		Options: []core.ConnectionOption{
			{Name: "None", Value: "none"},
			{Name: "Horizontal", Value: "horizontal"},
			{Name: "Vertical", Value: "vertical"},
		},
	},
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
	angle := ic.OptionalStringDefault("angle", "0", inputs)
	flip := ic.OptionalStringDefault("flip", "none", inputs)

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

	args := []string{inPath}
	if angle != "0" && angle != "" {
		args = append(args, "-rotate", angle)
	}
	switch flip {
	case "horizontal":
		args = append(args, "-flop")
	case "vertical":
		args = append(args, "-flip")
	}
	args = append(args, outPath)

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	stderr, err := ic.RunMagick(ctx, args...)
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
		"tool_result": fmt.Sprintf("Rotated %s°, flip=%s (%d bytes).", angle, flip, size),
		"image":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

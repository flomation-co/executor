// Package watermark composites an overlay image onto a base image. ImageMagick-backed.
package watermark

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
	Name         = "Watermark Image"
	Description  = "Overlay a watermark image onto another image at a chosen position and opacity"
	Website      = "https://www.flomation.co"
	Icon         = "layer-group"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Base image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "overlay", Type: core.ConnectionTypeString, Label: "Watermark image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "gravity", Type: core.ConnectionTypeString, Label: "Position", Value: "SouthEast",
		Options: []core.ConnectionOption{
			{Name: "Centre", Value: "Center"},
			{Name: "Top left", Value: "NorthWest"},
			{Name: "Top right", Value: "NorthEast"},
			{Name: "Bottom left", Value: "SouthWest"},
			{Name: "Bottom right", Value: "SouthEast"},
		},
	},
	{Name: "opacity", Type: core.ConnectionTypeInteger, Label: "Opacity (1–100)", Value: 100},
	{Name: "margin", Type: core.ConnectionTypeInteger, Label: "Margin (px)", Value: 10},
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
	overlay, err := ic.RequiredString("overlay", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	gravity := ic.OptionalStringDefault("gravity", "SouthEast", inputs)
	opacity := ic.OptionalInt("opacity", 100, inputs)
	if opacity < 1 {
		opacity = 1
	} else if opacity > 100 {
		opacity = 100
	}
	margin := ic.OptionalInt("margin", 10, inputs)
	if margin < 0 {
		margin = 0
	}

	basePath, _, err := flow.ResolveToLocalFile(image)
	if err != nil {
		return ic.ErrResult("could not read the base image: " + err.Error())
	}
	overlayPath, _, err := flow.ResolveToLocalFile(overlay)
	if err != nil {
		return ic.ErrResult("could not read the watermark image: " + err.Error())
	}
	ext := filepath.Ext(basePath)
	if ext == "" {
		ext = ".png"
	}
	outPath, err := flow.MediaScratchOutput(basePath, ext)
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	// Multiply the overlay's alpha channel to apply opacity, then composite it at
	// the requested gravity with a margin.
	frac := fmt.Sprintf("%.4f", float64(opacity)/100.0)
	geo := fmt.Sprintf("+%d+%d", margin, margin)
	stderr, err := ic.RunMagick(ctx,
		basePath,
		"(", overlayPath, "-alpha", "set", "-channel", "A", "-evaluate", "multiply", frac, "+channel", ")",
		"-gravity", gravity, "-geometry", geo, "-composite", outPath)
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
		"tool_result": fmt.Sprintf("Applied watermark at %s (%d%% opacity, %d bytes).", gravity, opacity, size),
		"image":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

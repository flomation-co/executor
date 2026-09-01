// Package psd_rasterise flattens a PSD/PSB to a web image (PNG/JPEG/WebP) via
// ImageMagick. It reads the EMBEDDED composite (input.psd[0]) — the merged
// preview Photoshop bakes in — rather than re-compositing layers, so blend
// modes and layer effects stay faithful. CMYK PSDs are converted to sRGB.
package psd_rasterise

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
	Name         = "Rasterise PSD"
	Description  = "Flatten a PSD to a PNG, JPEG or WebP image (from its embedded composite)"
	Website      = "https://www.flomation.co"
	Icon         = "image"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "psd", Type: core.ConnectionTypeString, Label: "PSD file (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "format", Type: core.ConnectionTypeString, Label: "Output format", Value: "png",
		Options: []core.ConnectionOption{
			{Name: "PNG (keeps transparency)", Value: "png"},
			{Name: "JPEG", Value: "jpg"},
			{Name: "WebP", Value: "webp"},
		},
	},
	{Name: "quality", Type: core.ConnectionTypeInteger, Label: "Quality (JPEG/WebP, 1–100)", Value: 90},
	{Name: "max_width", Type: core.ConnectionTypeInteger, Label: "Max width (px, 0 = original; only downscales)", Value: 0},
	{Name: "background", Type: core.ConnectionTypeString, Label: "Flatten onto colour (blank = keep transparency)", Placeholder: "#ffffff"},
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
	ref, err := ic.RequiredString("psd", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	format := ic.OptionalStringDefault("format", "png", inputs)
	quality := ic.OptionalInt("quality", 90, inputs)
	maxWidth := ic.OptionalInt("max_width", 0, inputs)
	background := ic.OptionalString("background", inputs)

	ext := format
	if ext == "jpeg" {
		ext = "jpg"
	}

	inPath, _, err := flow.ResolveToLocalFile(ref)
	if err != nil {
		return ic.ErrResult("could not read the input PSD: " + err.Error())
	}
	outPath, err := flow.MediaScratchOutput(inPath, ext)
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	// [0] selects the embedded flattened composite. -colorspace sRGB normalises
	// CMYK/other-mode PSDs for the web.
	args := []string{inPath + "[0]", "-colorspace", "sRGB"}
	if maxWidth > 0 {
		// The ">" flag downscales only — never enlarges a small PSD.
		args = append(args, "-resize", strconv.Itoa(maxWidth)+"x>")
	}
	// JPEG has no alpha; flatten onto the requested colour, or white by default.
	flatten := background
	if flatten == "" && (ext == "jpg") {
		flatten = "#ffffff"
	}
	if flatten != "" {
		args = append(args, "-background", flatten, "-flatten")
	}
	if quality > 0 && (ext == "jpg" || ext == "webp") {
		args = append(args, "-quality", strconv.Itoa(quality))
	}
	args = append(args, outPath)

	if out, err := ic.RunMagick(ctx, args...); err != nil {
		return ic.ErrResult("rasterise failed: " + err.Error() + ": " + ic.Tail(out, 200))
	}

	info, _ := ic.Identify(ctx, outPath)
	var w, h int
	if info != nil {
		w, h = info.Width, info.Height
	}
	var size int64
	if fi, e := os.Stat(outPath); e == nil {
		size = fi.Size()
	}

	imgRef, err := flow.EmitMediaFile(outPath)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Rasterised PSD to %s, %dx%d (%d bytes).", ext, w, h, size),
		"image":       imgRef,
		"width":       w,
		"height":      h,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

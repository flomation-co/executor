// Package adjust applies a single image effect (blur, sharpen, grayscale, etc.),
// backed by ImageMagick.
package adjust

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
	Name         = "Adjust Image"
	Description  = "Apply an effect: grayscale, sepia, blur, sharpen, brightness or contrast"
	Website      = "https://www.flomation.co"
	Icon         = "gauge"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{
		Name: "effect", Type: core.ConnectionTypeString, Label: "Effect", Value: "grayscale",
		Options: []core.ConnectionOption{
			{Name: "Grayscale", Value: "grayscale"},
			{Name: "Sepia", Value: "sepia"},
			{Name: "Blur", Value: "blur"},
			{Name: "Sharpen", Value: "sharpen"},
			{Name: "Brightness", Value: "brightness"},
			{Name: "Contrast", Value: "contrast"},
		},
	},
	{Name: "amount", Type: core.ConnectionTypeInteger, Label: "Amount (effect-dependent)", Value: 20},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// effectArgs maps an effect + amount to the magick operation args.
func effectArgs(effect string, amount int) []string {
	switch effect {
	case "sepia":
		return []string{"-sepia-tone", fmt.Sprintf("%d%%", clamp(amount, 0, 100, 80))}
	case "blur":
		return []string{"-blur", fmt.Sprintf("0x%d", clamp(amount, 0, 100, 5))}
	case "sharpen":
		return []string{"-sharpen", fmt.Sprintf("0x%d", clamp(amount, 0, 100, 5))}
	case "brightness":
		return []string{"-brightness-contrast", fmt.Sprintf("%dx0", clamp(amount, -100, 100, 20))}
	case "contrast":
		return []string{"-brightness-contrast", fmt.Sprintf("0x%d", clamp(amount, -100, 100, 20))}
	default: // grayscale
		return []string{"-colorspace", "Gray"}
	}
}

func clamp(v, lo, hi, def int) int {
	if v == 0 && def != 0 {
		// treat an unset/zero amount as the sensible default for the effect
		v = def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	image, err := ic.RequiredString("image", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	effect := ic.OptionalStringDefault("effect", "grayscale", inputs)
	amount := ic.OptionalInt("amount", 20, inputs)

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

	args := append([]string{inPath}, effectArgs(effect, amount)...)
	args = append(args, outPath)
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
		"tool_result": fmt.Sprintf("Applied %s (%d bytes).", effect, size),
		"image":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}

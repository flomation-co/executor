// Package psd_render_text personalises a PSD by replacing a named text layer's
// content, WITHOUT a real type engine (Photoshop/Photopea). It is a gg-hybrid:
//
//  1. locate the target text layer by name and read its bounding box (identify);
//  2. reconstruct the background = the design with the composite AND the target
//     layer removed, re-flattened (ImageMagick);
//  3. draw the new text into that box with the pure-Go gg renderer + embedded
//     Poppins fonts (reused from the graphics/ category).
//
// The deliverable is a rasterised image, never a re-saved .psd. Honest limits:
// we render with OUR fonts (not the PSD's embedded typeface unless it's Poppins),
// and dropping the text layer re-composites the rest, so exotic blend modes /
// adjustment layers approximate.
package psd_render_text

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/fogleman/gg"

	core "flomation.app/automate/executor"
	gc "flomation.app/automate/executor/actions/graphics"
	ic "flomation.app/automate/executor/actions/image"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Personalise PSD Text"
	Description  = "Replace a named text layer in a PSD and render the result as an image"
	Website      = "https://www.flomation.co"
	Icon         = "pen"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "psd", Type: core.ConnectionTypeString, Label: "PSD file (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "layer_name", Type: core.ConnectionTypeString, Label: "Text layer name (exact, from PSD Info)", Placeholder: "headline", Required: true},
	{Name: "text", Type: core.ConnectionTypeText, Label: "New text", Placeholder: "Hello Jane", Required: true},
	{
		Name: "font", Type: core.ConnectionTypeString, Label: "Font", Value: "poppins-bold",
		Options: []core.ConnectionOption{
			{Name: "Poppins Bold", Value: "poppins-bold"},
			{Name: "Poppins SemiBold", Value: "poppins-semibold"},
			{Name: "Poppins Regular", Value: "poppins-regular"},
		},
	},
	{Name: "font_size", Type: core.ConnectionTypeInteger, Label: "Font size (px, 0 = auto-fit to the layer box)", Value: 0},
	{Name: "colour", Type: core.ConnectionTypeString, Label: "Text colour", Value: "#000000", Placeholder: "#000000, flomation-teal"},
	{
		Name: "align", Type: core.ConnectionTypeString, Label: "Alignment", Value: "center",
		Options: []core.ConnectionOption{
			{Name: "Left", Value: "left"},
			{Name: "Centre", Value: "center"},
			{Name: "Right", Value: "right"},
		},
	},
	{
		Name: "format", Type: core.ConnectionTypeString, Label: "Output format", Value: "png",
		Options: []core.ConnectionOption{
			{Name: "PNG", Value: "png"},
			{Name: "JPEG", Value: "jpg"},
			{Name: "WebP", Value: "webp"},
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

const sep = "\x1f"

type layer struct {
	idx        int
	name       string
	x, y, w, h int
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ref, err := ic.RequiredString("psd", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	layerName, err := ic.RequiredString("layer_name", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	text, err := ic.RequiredString("text", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	font := ic.OptionalStringDefault("font", "poppins-bold", inputs)
	fontSize := ic.OptionalInt("font_size", 0, inputs)
	colour := ic.OptionalStringDefault("colour", "#000000", inputs)
	align := ic.OptionalStringDefault("align", "center", inputs)
	format := ic.OptionalStringDefault("format", "png", inputs)

	inPath, _, err := flow.ResolveToLocalFile(ref)
	if err != nil {
		return ic.ErrResult("could not read the input PSD: " + err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	// 1. Read the layer list; find the target by name; note whether scene 0 is
	//    the (unnamed) composite so we know to drop it from the background.
	layers, compositePresent, err := readLayers(ctx, inPath)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	target, ok := findLayer(layers, layerName)
	if !ok {
		return ic.ErrResult(fmt.Sprintf("no layer named %q — available: %s", layerName, layerNames(layers)))
	}

	// 2. Reconstruct the background: drop the composite [0] and the target layer,
	//    re-flatten the rest so the space the old text occupied is clean.
	bgPath, err := flow.MediaScratchFile("png")
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	del := []string{}
	if compositePresent {
		// Removing scene 0 shifts every later index down by one.
		del = append(del, "-delete", "0", "-delete", strconv.Itoa(target.idx-1))
	} else {
		del = append(del, "-delete", strconv.Itoa(target.idx))
	}
	args := append([]string{inPath}, del...)
	args = append(args, "-background", "none", "-layers", "flatten", bgPath)
	if out, err := ic.RunMagick(ctx, args...); err != nil {
		return ic.ErrResult("could not rebuild the background: " + err.Error() + ": " + ic.Tail(out, 200))
	}

	// 3. Draw the new text into the target's box with the embedded-font gg renderer.
	bg, err := gg.LoadPNG(bgPath)
	if err != nil {
		return ic.ErrResult("could not load the reconstructed background: " + err.Error())
	}
	dc := gg.NewContextForImage(bg)
	size := fitFontSize(font, text, fontSize, target)
	face, err := gc.FontFace(font, size)
	if err != nil {
		return ic.ErrResult("could not load the font: " + err.Error())
	}
	dc.SetFontFace(face)
	gc.SetColour(dc, colour, 1)
	ax, tx := anchorX(align, target)
	ty := float64(target.y) + float64(target.h)/2
	dc.DrawStringAnchored(text, tx, ty, ax, 0.5)

	// Save as PNG, then convert to the requested container if needed.
	renderedPNG, err := flow.MediaScratchFile("png")
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	if err := dc.SavePNG(renderedPNG); err != nil {
		return ic.ErrResult("could not save the rendered image: " + err.Error())
	}
	outPath := renderedPNG
	ext := normaliseExt(format)
	if ext != "png" {
		converted, err := flow.MediaScratchFile(ext)
		if err != nil {
			return ic.ErrResult(err.Error())
		}
		cargs := []string{renderedPNG}
		if ext == "jpg" {
			cargs = append(cargs, "-background", "#ffffff", "-flatten")
		}
		cargs = append(cargs, converted)
		if out, err := ic.RunMagick(ctx, cargs...); err != nil {
			return ic.ErrResult("could not convert output: " + err.Error() + ": " + ic.Tail(out, 200))
		}
		outPath = converted
	}

	var size64 int64
	if fi, e := os.Stat(outPath); e == nil {
		size64 = fi.Size()
	}
	imgRef, err := flow.EmitMediaFile(outPath)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Replaced layer %q and rendered %dx%d (%d bytes).", target.name, dc.Width(), dc.Height(), size64),
		"image":       imgRef,
		"width":       dc.Width(),
		"height":      dc.Height(),
		"size_bytes":  size64,
		"success":     true,
	}, nil
}

// readLayers runs identify and parses the scene list. compositePresent is true
// when scene 0 is the unnamed, full-canvas flattened preview Photoshop embeds.
func readLayers(ctx context.Context, path string) (ls []layer, compositePresent bool, err error) {
	format := strings.Join([]string{"%s", "%[label]", "%w", "%h", "%X", "%Y"}, sep) + "\n"
	out, err := ic.IdentifyFormat(ctx, format, path)
	if err != nil {
		return nil, false, fmt.Errorf("could not read the PSD layers: %v: %s", err, ic.Tail(out, 200))
	}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, sep)
		if len(f) < 6 {
			continue
		}
		ls = append(ls, layer{
			idx:  atoi(f[0]),
			name: strings.TrimSpace(f[1]),
			w:    atoi(f[2]),
			h:    atoi(f[3]),
			x:    atoi(f[4]),
			y:    atoi(f[5]),
		})
	}
	if len(ls) == 0 {
		return nil, false, fmt.Errorf("the file contains no readable layers (is it a valid PSD?)")
	}
	compositePresent = ls[0].idx == 0 && ls[0].name == ""
	return ls, compositePresent, nil
}

func findLayer(ls []layer, name string) (layer, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, l := range ls {
		if strings.ToLower(l.name) == want && l.name != "" {
			return l, true
		}
	}
	return layer{}, false
}

// fitFontSize returns the explicit size when > 0, else the largest size whose
// rendered text fits inside the layer box (95% width, 90% height headroom).
func fitFontSize(font, text string, explicit int, box layer) float64 {
	if explicit > 0 {
		return float64(explicit)
	}
	const ref = 100.0
	face, err := gc.FontFace(font, ref)
	if err != nil {
		return 24
	}
	dc := gg.NewContext(1, 1)
	dc.SetFontFace(face)
	w, h := dc.MeasureString(text)
	if w <= 0 || h <= 0 {
		return 24
	}
	byWidth := (float64(box.w) * 0.95) / w * ref
	byHeight := (float64(box.h) * 0.90) / h * ref
	size := math.Min(byWidth, byHeight)
	if size < 6 {
		size = 6
	}
	return size
}

// anchorX maps the alignment to a gg anchor (0/0.5/1) and the x within the box.
func anchorX(align string, box layer) (anchor, x float64) {
	switch align {
	case "left":
		return 0, float64(box.x)
	case "right":
		return 1, float64(box.x + box.w)
	default: // center
		return 0.5, float64(box.x) + float64(box.w)/2
	}
}

func layerNames(ls []layer) string {
	var names []string
	for _, l := range ls {
		if l.name != "" {
			names = append(names, l.name)
		}
	}
	if len(names) == 0 {
		return "(no named layers)"
	}
	return strings.Join(names, ", ")
}

func normaliseExt(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jpg", "jpeg":
		return "jpg"
	case "webp":
		return "webp"
	default:
		return "png"
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

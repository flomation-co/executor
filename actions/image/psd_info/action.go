// Package psd_info reads a PSD/PSB's structure via ImageMagick's identify:
// canvas size, colour mode, and the per-layer index/name/bounding-box list.
// Read-only. The layer list is the metadata backbone the gg-hybrid text
// templating action (image/psd_render_text) reads to locate a layer by name.
package psd_info

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	ic "flomation.app/automate/executor/actions/image"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "PSD Info"
	Description  = "Read a PSD's canvas size, colour mode and layer names/positions"
	Website      = "https://www.flomation.co"
	Icon         = "layer-group"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "psd", Type: core.ConnectionTypeString, Label: "PSD file (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "layers", Type: core.ConnectionTypeString, Label: "Layers (JSON array)"},
	{Name: "layer_count", Type: core.ConnectionTypeInteger, Label: "Layer count"},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Canvas width (px)"},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Canvas height (px)"},
	{Name: "colour_mode", Type: core.ConnectionTypeString, Label: "Colour mode"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// sep is an ASCII Unit Separator — a delimiter that cannot occur in a layer
// name, so we can split identify's per-field output unambiguously.
const sep = "\x1f"

// Layer is one entry in the PSD's scene list. Index 0 is normally the flattened
// composite Photoshop embeds; named entries are the real layers.
type Layer struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ref, err := ic.RequiredString("psd", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	inPath, _, err := flow.ResolveToLocalFile(ref)
	if err != nil {
		return ic.ErrResult("could not read the input PSD: " + err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	// One line per scene: index, name, layer size, layer offset, colourspace.
	// (%W/%H page size is unreliable per-layer — a layer reports its own size,
	// not the canvas — so the canvas is taken from scene 0, the composite.)
	format := strings.Join([]string{"%s", "%[label]", "%w", "%h", "%X", "%Y", "%[colorspace]"}, sep) + "\n"
	out, err := ic.IdentifyFormat(ctx, format, inPath)
	if err != nil {
		return ic.ErrResult("could not read the PSD layers: " + err.Error() + ": " + ic.Tail(out, 200))
	}

	var (
		layers           []Layer
		canvasW, canvasH int
		colourMode       string
	)
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, sep)
		if len(f) < 7 {
			continue
		}
		l := Layer{
			Index:  atoi(f[0]),
			Name:   f[1],
			Width:  atoi(f[2]),
			Height: atoi(f[3]),
			X:      atoi(f[4]),
			Y:      atoi(f[5]),
		}
		layers = append(layers, l)
		// Scene 0 is the flattened composite: its image size IS the canvas, and
		// its colourspace is the document's colour mode.
		if l.Index == 0 {
			canvasW, canvasH, colourMode = l.Width, l.Height, f[6]
		}
	}
	if len(layers) == 0 {
		return ic.ErrResult("the file contains no readable layers (is it a valid PSD?)")
	}
	// Fallback when there's no composite scene 0 (a PSD saved without
	// maximise-compatibility): take the canvas as the bounding extent of layers.
	if canvasW == 0 || canvasH == 0 {
		for _, l := range layers {
			if l.X+l.Width > canvasW {
				canvasW = l.X + l.Width
			}
			if l.Y+l.Height > canvasH {
				canvasH = l.Y + l.Height
			}
			if colourMode == "" {
				colourMode = "sRGB"
			}
		}
	}

	b, _ := json.Marshal(layers)
	named := 0
	for _, l := range layers {
		if strings.TrimSpace(l.Name) != "" {
			named++
		}
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("PSD %dx%d, %s, %d layers (%d named).", canvasW, canvasH, colourMode, len(layers), named),
		"layers":      string(b),
		"layer_count": len(layers),
		"width":       canvasW,
		"height":      canvasH,
		"colour_mode": colourMode,
		"success":     true,
	}, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

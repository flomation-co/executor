// Package psd_extract_layers exports individual PSD layers as separate PNG
// images (preserving each layer's transparency). By default it exports every
// named layer; an optional comma-separated names filter narrows it. Each layer
// is emitted as a media reference in the returned JSON array.
package psd_extract_layers

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
	Name         = "Extract PSD Layers"
	Description  = "Export individual PSD layers as separate PNG images"
	Website      = "https://www.flomation.co"
	Icon         = "object-group"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "psd", Type: core.ConnectionTypeString, Label: "PSD file (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "names", Type: core.ConnectionTypeString, Label: "Layer names to extract (comma-separated, blank = all named)", Placeholder: "logo, headline"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "layers", Type: core.ConnectionTypeString, Label: "Extracted layers (JSON array)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Layers extracted"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

const sep = "\x1f"

type extracted struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Ref    string `json:"ref"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ref, err := ic.RequiredString("psd", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	wanted := parseNames(ic.OptionalString("names", inputs))

	inPath, _, err := flow.ResolveToLocalFile(ref)
	if err != nil {
		return ic.ErrResult("could not read the input PSD: " + err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	format := strings.Join([]string{"%s", "%[label]", "%w", "%h"}, sep) + "\n"
	out, err := ic.IdentifyFormat(ctx, format, inPath)
	if err != nil {
		return ic.ErrResult("could not read the PSD layers: " + err.Error() + ": " + ic.Tail(out, 200))
	}

	type scene struct {
		idx, w, h int
		name      string
	}
	var scenes []scene
	anyNamed := false
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, sep)
		if len(f) < 4 {
			continue
		}
		s := scene{idx: atoi(f[0]), name: strings.TrimSpace(f[1]), w: atoi(f[2]), h: atoi(f[3])}
		scenes = append(scenes, s)
		if s.name != "" {
			anyNamed = true
		}
	}

	var results []extracted
	for _, s := range scenes {
		if !shouldExtract(s.idx, s.name, wanted, anyNamed) {
			continue
		}
		layerPath, err := flow.MediaScratchFile("png")
		if err != nil {
			return ic.ErrResult(err.Error())
		}
		// [idx] selects one scene; -background none keeps the layer's alpha.
		if eout, err := ic.RunMagick(ctx, inPath+"["+strconv.Itoa(s.idx)+"]", "-background", "none", layerPath); err != nil {
			return ic.ErrResult(fmt.Sprintf("could not export layer %d (%q): %v: %s", s.idx, s.name, err, ic.Tail(eout, 150)))
		}
		lref, err := flow.EmitMediaFile(layerPath)
		if err != nil {
			return ic.ErrResult(err.Error())
		}
		results = append(results, extracted{Index: s.idx, Name: s.name, Ref: lref, Width: s.w, Height: s.h})
	}

	if len(results) == 0 {
		return ic.ErrResult("no matching layers to extract")
	}
	b, _ := json.Marshal(results)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Extracted %d layer(s).", len(results)),
		"layers":      string(b),
		"count":       len(results),
		"success":     true,
	}, nil
}

// shouldExtract decides whether a scene is in scope. With an explicit names
// filter, only matching names qualify. Otherwise: every named layer; but if the
// PSD has no named layers at all, every scene except index 0 (the composite).
func shouldExtract(idx int, name string, wanted map[string]bool, anyNamed bool) bool {
	if len(wanted) > 0 {
		return name != "" && wanted[strings.ToLower(name)]
	}
	if anyNamed {
		return name != ""
	}
	return idx != 0
}

func parseNames(csv string) map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Split(csv, ",") {
		if t := strings.ToLower(strings.TrimSpace(n)); t != "" {
			m[t] = true
		}
	}
	return m
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// Package palette extracts an image's dominant colours, backed by ImageMagick's
// colour quantisation + histogram. Read-only.
package palette

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	ic "flomation.app/automate/executor/actions/image"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Image Colour Palette"
	Description  = "Extract an image's dominant colours as hex codes"
	Website      = "https://www.flomation.co"
	Icon         = "chart-pie"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Number of colours", Value: 5},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "colours", Type: core.ConnectionTypeObject, Label: "Colours (hex array)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Colours found"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

var hexRe = regexp.MustCompile(`#[0-9A-Fa-f]{6}`)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	image, err := ic.RequiredString("image", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	count := ic.OptionalInt("count", 5, inputs)
	if count < 1 {
		count = 1
	} else if count > 64 {
		count = 64
	}

	inPath, _, err := flow.ResolveToLocalFile(image)
	if err != nil {
		return ic.ErrResult("could not read the input image: " + err.Error())
	}
	bin, err := ic.MagickPath()
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	// Quantise to `count` colours on a downscaled copy, then print the histogram.
	// #nosec G204 -- inPath is workspace-confined; the rest are constants.
	out, err := exec.CommandContext(ctx, bin, inPath,
		"-resize", "200x200", "+dither", "-colors", strconv.Itoa(count),
		"-depth", "8", "-format", "%c", "histogram:info:-").Output()
	if err != nil {
		return ic.ErrResult("magick failed to read the palette: " + err.Error())
	}

	seen := map[string]bool{}
	var colours []string
	for _, m := range hexRe.FindAllString(string(out), -1) {
		u := strings.ToUpper(m)
		if !seen[u] {
			seen[u] = true
			colours = append(colours, u)
		}
	}
	if len(colours) == 0 {
		return ic.ErrResult("no colours could be read from the image")
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Top colours: %s", strings.Join(colours, ", ")),
		"colours":     colours,
		"count":       len(colours),
		"success":     true,
	}, nil
}

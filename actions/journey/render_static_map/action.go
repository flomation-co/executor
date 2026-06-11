package journey_render_static_map

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Render Static Map"
	Description  = "Generate a PNG of a route from an encoded polyline, sized for inline display or PDF embedding."
	Website      = "https://www.flomation.co"
	Icon         = "route+image"
	Date         = "11/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "provider",
		Type:        core.ConnectionTypeString,
		Label:       "Map Provider",
		Placeholder: "google",
		Options: []core.ConnectionOption{
			{Name: "Google Maps", Value: "google"},
			{Name: "Mapbox", Value: "mapbox"},
		},
		Required: true,
	},
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeString,
		Label:       "Provider API Key",
		Placeholder: "${secrets.GOOGLE_MAPS_API_KEY}",
		Required:    true,
	},
	{
		Name:        "polyline",
		Type:        core.ConnectionTypeString,
		Label:       "Encoded polyline (from calculate_route)",
		Placeholder: "${parent.polyline}",
		Required:    true,
	},
	{
		Name:        "width",
		Type:        core.ConnectionTypeString,
		Label:       "Width (pixels)",
		Placeholder: "600",
	},
	{
		Name:        "height",
		Type:        core.ConnectionTypeString,
		Label:       "Height (pixels)",
		Placeholder: "400",
	},
	{
		Name:        "zoom",
		Type:        core.ConnectionTypeString,
		Label:       "Zoom (0 = auto-fit)",
		Placeholder: "0",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "image_base64", Type: core.ConnectionTypeString, Label: "Image (base64-encoded PNG)"},
	{Name: "mime_type", Type: core.ConnectionTypeString, Label: "MIME type"},
	{Name: "size_kb", Type: core.ConnectionTypeString, Label: "Image size (KB)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	provider, err := journey.ProviderInput(inputs)
	if err != nil {
		return nil, err
	}
	polyline, err := journey.RequiredString("polyline", inputs)
	if err != nil {
		return nil, err
	}

	width := parseIntDefault(journey.OptionalString("width", inputs), 600)
	height := parseIntDefault(journey.OptionalString("height", inputs), 400)
	zoom := parseIntDefault(journey.OptionalString("zoom", inputs), 0)

	img, mime, err := provider.RenderStaticMap(journey.StaticMapParams{
		Polyline: polyline,
		Width:    width,
		Height:   height,
		Zoom:     zoom,
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to render static map: %s", err.Error()),
			"success":     false,
			"error":       err.Error(),
		}, err
	}

	encoded := base64.StdEncoding.EncodeToString(img)
	sizeKB := float64(len(img)) / 1024.0

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Rendered %dx%d %s static map (%.1f KB).", width, height, mime, sizeKB),
		"image_base64": encoded,
		"mime_type":    mime,
		"size_kb":      fmt.Sprintf("%.1f", sizeKB),
		"success":      true,
		"error":        "",
	}, nil
}

func parseIntDefault(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}

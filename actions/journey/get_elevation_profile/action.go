package journey_get_elevation_profile

import (
	"encoding/json"
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Elevation Profile"
	Description  = "Sample elevations along a route polyline and return total ascent, descent, min and max."
	Website      = "https://www.flomation.co"
	Icon         = "route+mountain"
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
		Name:        "sample_count",
		Type:        core.ConnectionTypeString,
		Label:       "Number of samples (max 512)",
		Placeholder: "100",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "total_ascent_metres", Type: core.ConnectionTypeString, Label: "Total ascent (metres)"},
	{Name: "total_descent_metres", Type: core.ConnectionTypeString, Label: "Total descent (metres)"},
	{Name: "min_elevation_metres", Type: core.ConnectionTypeString, Label: "Minimum elevation (metres)"},
	{Name: "max_elevation_metres", Type: core.ConnectionTypeString, Label: "Maximum elevation (metres)"},
	{Name: "samples", Type: core.ConnectionTypeObject, Label: "Elevation samples"},
	{Name: "profile", Type: core.ConnectionTypeObject, Label: "Full elevation profile JSON"},
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

	sampleCount := 0
	if s := journey.OptionalString("sample_count", inputs); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			sampleCount = v
		}
	}

	result, err := provider.GetElevationProfile(journey.ElevationParams{
		Polyline:    polyline,
		SampleCount: sampleCount,
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to fetch elevation profile: %s", err.Error()),
			"success":     false,
			"error":       err.Error(),
		}, err
	}

	resultJSON, _ := json.Marshal(result)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Elevation profile: +%.0fm ascent, -%.0fm descent across %d samples (range %.0f-%.0fm).",
			result.TotalAscentMetres, result.TotalDescentMetres, len(result.Samples),
			result.MinElevationMetres, result.MaxElevationMetres),
		"total_ascent_metres":  fmt.Sprintf("%.0f", result.TotalAscentMetres),
		"total_descent_metres": fmt.Sprintf("%.0f", result.TotalDescentMetres),
		"min_elevation_metres": fmt.Sprintf("%.0f", result.MinElevationMetres),
		"max_elevation_metres": fmt.Sprintf("%.0f", result.MaxElevationMetres),
		"samples":              result.Samples,
		"profile":              json.RawMessage(resultJSON),
		"success":              true,
		"error":                "",
	}, nil
}

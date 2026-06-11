package journey_get_travel_time

import (
	"fmt"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Travel Time"
	Description  = "Quickly get the distance and duration between two locations without computing a full route."
	Website      = "https://www.flomation.co"
	Icon         = "route+clock"
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
		Name:        "origin",
		Type:        core.ConnectionTypeString,
		Label:       "Origin (address or lat,lng)",
		Placeholder: "London, UK",
		Required:    true,
	},
	{
		Name:        "destination",
		Type:        core.ConnectionTypeString,
		Label:       "Destination (address or lat,lng)",
		Placeholder: "Manchester, UK",
		Required:    true,
	},
	{
		Name:        "mode",
		Type:        core.ConnectionTypeString,
		Label:       "Travel Mode",
		Placeholder: "driving",
		Options: []core.ConnectionOption{
			{Name: "Driving", Value: "driving"},
			{Name: "Walking", Value: "walking"},
			{Name: "Cycling", Value: "cycling"},
			{Name: "Transit", Value: "transit"},
		},
	},
	{
		Name:        "depart_at",
		Type:        core.ConnectionTypeDateTime,
		Label:       "Departure time",
		Placeholder: "leave blank for 'now'",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "distance_metres", Type: core.ConnectionTypeString, Label: "Distance (metres)"},
	{Name: "distance_miles", Type: core.ConnectionTypeString, Label: "Distance (miles)"},
	{Name: "duration_seconds", Type: core.ConnectionTypeString, Label: "Duration (seconds)"},
	{Name: "duration_friendly", Type: core.ConnectionTypeString, Label: "Duration (friendly)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	provider, err := journey.ProviderInput(inputs)
	if err != nil {
		return nil, err
	}
	origin, err := journey.RequiredString("origin", inputs)
	if err != nil {
		return nil, err
	}
	destination, err := journey.RequiredString("destination", inputs)
	if err != nil {
		return nil, err
	}
	departAt, err := journey.OptionalTime("depart_at", inputs)
	if err != nil {
		return nil, err
	}
	mode := journey.OptionalString("mode", inputs)

	tt, err := provider.GetTravelTime(origin, destination, mode, departAt)
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to fetch travel time from %s to %s: %s", origin, destination, err.Error()),
			"success":     false,
			"error":       err.Error(),
		}, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("From %s to %s: %.1f miles, %s.",
			origin, destination, tt.DistanceMiles, tt.DurationFriendly),
		"distance_metres":   fmt.Sprintf("%.0f", tt.DistanceMetres),
		"distance_miles":    fmt.Sprintf("%.2f", tt.DistanceMiles),
		"duration_seconds":  fmt.Sprintf("%.0f", tt.DurationSeconds),
		"duration_friendly": tt.DurationFriendly,
		"success":           true,
		"error":             "",
	}, nil
}

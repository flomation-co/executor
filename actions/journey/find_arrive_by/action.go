package journey_find_arrive_by

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Find Departure Time"
	Description  = "Calculate when to leave to arrive by a given time, accounting for traffic at that hour."
	Website      = "https://www.flomation.co"
	Icon         = "route+hourglass-end"
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
		Type:        core.ConnectionTypeSecret,
		Label:       "Provider API Key",
		Placeholder: "${secrets.GOOGLE_MAPS_API_KEY}",
		Required:    true,
	},
	{
		Name:        "origin",
		Type:        core.ConnectionTypeSecret,
		Label:       "Origin",
		Placeholder: "Home",
		Required:    true,
	},
	{
		Name:        "destination",
		Type:        core.ConnectionTypeString,
		Label:       "Destination",
		Placeholder: "Office",
		Required:    true,
	},
	{
		Name:        "arrive_by",
		Type:        core.ConnectionTypeDateTime,
		Label:       "Arrive by",
		Placeholder: "2026-06-12T09:00",
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "recommended_departure", Type: core.ConnectionTypeString, Label: "Recommended departure (RFC3339)"},
	{Name: "estimated_duration_seconds", Type: core.ConnectionTypeString, Label: "Estimated duration (seconds)"},
	{Name: "estimated_duration_friendly", Type: core.ConnectionTypeString, Label: "Estimated duration (friendly)"},
	{Name: "distance_miles", Type: core.ConnectionTypeString, Label: "Distance (miles)"},
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
	arriveBy, err := journey.OptionalTime("arrive_by", inputs)
	if err != nil {
		return nil, err
	}
	if arriveBy == nil {
		return nil, fmt.Errorf("journey: input %q is required", "arrive_by")
	}
	mode := journey.OptionalString("mode", inputs)

	// Two-pass estimate: first call uses no depart_at (free-flow estimate),
	// gives us a rough candidate departure. Second call uses that candidate
	// as depart_at so traffic at the actual hour is factored in. One refinement
	// pass is usually enough — further passes converge within seconds.
	first, err := provider.GetTravelTime(origin, destination, mode, nil)
	if err != nil {
		return errResult(origin, destination, err)
	}
	candidate := arriveBy.Add(-time.Duration(first.DurationSeconds) * time.Second)

	second, err := provider.GetTravelTime(origin, destination, mode, &candidate)
	if err != nil {
		return errResult(origin, destination, err)
	}
	recommended := arriveBy.Add(-time.Duration(second.DurationSeconds) * time.Second)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Leave at %s to arrive by %s (%.1f miles, %s).",
			recommended.Format(time.RFC3339), arriveBy.Format(time.RFC3339),
			second.DistanceMiles, second.DurationFriendly),
		"recommended_departure":        recommended.Format(time.RFC3339),
		"estimated_duration_seconds":   fmt.Sprintf("%.0f", second.DurationSeconds),
		"estimated_duration_friendly":  second.DurationFriendly,
		"distance_miles":               fmt.Sprintf("%.2f", second.DistanceMiles),
		"success":                      true,
		"error":                        "",
	}, nil
}

func errResult(origin, destination string, err error) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Failed to estimate travel time from %s to %s: %s", origin, destination, err.Error()),
		"success":     false,
		"error":       err.Error(),
	}, err
}

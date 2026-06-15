package journey_optimise_route

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Optimise Route"
	Description  = "Find the shortest visit order for a set of stops, with optional fixed start and end anchors."
	Website      = "https://www.flomation.co"
	Icon         = "route+arrows-spin"
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
		Name:        "start",
		Type:        core.ConnectionTypeSecret,
		Label:       "Start (optional anchor)",
		Placeholder: "London depot, UK",
	},
	{
		Name:        "end",
		Type:        core.ConnectionTypeString,
		Label:       "End (optional anchor)",
		Placeholder: "Leave blank to end at last stop",
	},
	{
		Name:        "stops",
		Type:        core.ConnectionTypeString,
		Label:       "Stops (comma- or pipe-separated)",
		Placeholder: "Birmingham|Coventry|Stoke|Manchester",
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
		Name:        "optimise_for",
		Type:        core.ConnectionTypeString,
		Label:       "Optimise For",
		Placeholder: "duration",
		Options: []core.ConnectionOption{
			{Name: "Shortest time", Value: "duration"},
			{Name: "Shortest distance", Value: "distance"},
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
	{Name: "ordered_stops", Type: core.ConnectionTypeObject, Label: "Stops in optimal order"},
	{Name: "waypoint_order", Type: core.ConnectionTypeObject, Label: "Original indices in visit order"},
	{Name: "total_distance_miles", Type: core.ConnectionTypeString, Label: "Total distance (miles)"},
	{Name: "total_duration_friendly", Type: core.ConnectionTypeString, Label: "Total duration (friendly)"},
	{Name: "legs", Type: core.ConnectionTypeObject, Label: "Per-leg breakdown"},
	{Name: "optimisation", Type: core.ConnectionTypeObject, Label: "Full optimisation JSON"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	provider, err := journey.ProviderInput(inputs)
	if err != nil {
		return nil, err
	}
	stops := journey.OptionalCSV("stops", inputs)
	if len(stops) == 0 {
		return nil, fmt.Errorf("journey: input %q is required", "stops")
	}
	departAt, err := journey.OptionalTime("depart_at", inputs)
	if err != nil {
		return nil, err
	}

	params := journey.OptimiseParams{
		Start:       journey.OptionalString("start", inputs),
		End:         journey.OptionalString("end", inputs),
		Stops:       stops,
		Mode:        journey.OptionalString("mode", inputs),
		OptimiseFor: journey.OptionalString("optimise_for", inputs),
		DepartAt:    departAt,
	}

	result, err := provider.OptimiseRoute(params)
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to optimise route: %s", err.Error()),
			"success":     false,
			"error":       err.Error(),
		}, err
	}

	resultJSON, _ := json.Marshal(result)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Optimised %d stops: %.1f miles, %s. Visit order: %v",
			len(result.OrderedStops), result.TotalDistanceMiles, result.TotalDurationFriendly, result.OrderedStops),
		"ordered_stops":          result.OrderedStops,
		"waypoint_order":         result.WaypointOrder,
		"total_distance_miles":   fmt.Sprintf("%.2f", result.TotalDistanceMiles),
		"total_duration_friendly": result.TotalDurationFriendly,
		"legs":                   result.Legs,
		"optimisation":           json.RawMessage(resultJSON),
		"success":                true,
		"error":                  "",
	}, nil
}

package journey_compare_departure_times

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Compare Departure Times"
	Description  = "Run the same route at multiple departure times and surface rush-hour deltas."
	Website      = "https://www.flomation.co"
	Icon         = "route+clock-rotate-left"
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
		Label:       "Origin",
		Placeholder: "London, UK",
		Required:    true,
	},
	{
		Name:        "destination",
		Type:        core.ConnectionTypeString,
		Label:       "Destination",
		Placeholder: "Manchester, UK",
		Required:    true,
	},
	{
		Name:        "waypoints",
		Type:        core.ConnectionTypeString,
		Label:       "Waypoints (comma- or pipe-separated)",
		Placeholder: "Birmingham|Stoke",
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
		Name:        "departure_times",
		Type:        core.ConnectionTypeString,
		Label:       "Departure times (comma-separated RFC3339)",
		Placeholder: "2026-06-12T07:00:00Z,2026-06-12T08:00:00Z,2026-06-12T17:00:00Z",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "comparisons", Type: core.ConnectionTypeObject, Label: "Per-departure breakdown"},
	{Name: "best_departure", Type: core.ConnectionTypeString, Label: "Fastest departure time (RFC3339)"},
	{Name: "best_duration_friendly", Type: core.ConnectionTypeString, Label: "Fastest duration"},
	{Name: "worst_departure", Type: core.ConnectionTypeString, Label: "Slowest departure time (RFC3339)"},
	{Name: "worst_duration_friendly", Type: core.ConnectionTypeString, Label: "Slowest duration"},
	{Name: "delta_seconds", Type: core.ConnectionTypeString, Label: "Time difference (seconds)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type comparison struct {
	DepartAt         string  `json:"depart_at"`
	DurationSeconds  float64 `json:"duration_seconds"`
	DurationFriendly string  `json:"duration_friendly"`
	DistanceMiles    float64 `json:"distance_miles"`
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
	timesRaw, err := journey.RequiredString("departure_times", inputs)
	if err != nil {
		return nil, err
	}

	times, err := parseTimes(timesRaw)
	if err != nil {
		return nil, err
	}

	waypoints := journey.OptionalCSV("waypoints", inputs)
	mode := journey.OptionalString("mode", inputs)

	comparisons := make([]comparison, 0, len(times))
	var best, worst *comparison
	for _, t := range times {
		tCopy := t
		route, rErr := provider.CalculateRoute(journey.RouteParams{
			Origin:      origin,
			Destination: destination,
			Waypoints:   waypoints,
			Mode:        mode,
			DepartAt:    &tCopy,
		})
		if rErr != nil {
			return map[string]interface{}{
				"tool_result": fmt.Sprintf("Failed to calculate route at %s: %s", t.Format(time.RFC3339), rErr.Error()),
				"success":     false,
				"error":       rErr.Error(),
			}, rErr
		}
		c := comparison{
			DepartAt:         t.Format(time.RFC3339),
			DurationSeconds:  route.DurationSeconds,
			DurationFriendly: route.DurationFriendly,
			DistanceMiles:    route.DistanceMiles,
		}
		comparisons = append(comparisons, c)

		if best == nil || c.DurationSeconds < best.DurationSeconds {
			cBest := c
			best = &cBest
		}
		if worst == nil || c.DurationSeconds > worst.DurationSeconds {
			cWorst := c
			worst = &cWorst
		}
	}

	delta := worst.DurationSeconds - best.DurationSeconds

	cmpJSON, _ := json.Marshal(comparisons)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Compared %d departure times: best %s (%s), worst %s (%s), delta %s.",
			len(comparisons), best.DepartAt, best.DurationFriendly, worst.DepartAt, worst.DurationFriendly,
			journey.FriendlyDuration(delta)),
		"comparisons":             json.RawMessage(cmpJSON),
		"best_departure":          best.DepartAt,
		"best_duration_friendly":  best.DurationFriendly,
		"worst_departure":         worst.DepartAt,
		"worst_duration_friendly": worst.DurationFriendly,
		"delta_seconds":           fmt.Sprintf("%.0f", delta),
		"success":                 true,
		"error":                   "",
	}, nil
}

// parseTimes accepts comma- or pipe-separated RFC3339 (or `2006-01-02T15:04`)
// timestamps. Bare HH:MM values are NOT supported here — the caller needs to
// know the date because the action could span midnight.
func parseTimes(raw string) ([]time.Time, error) {
	sep := ","
	if strings.Contains(raw, "|") && !strings.Contains(raw, ",") {
		sep = "|"
	}
	parts := strings.Split(raw, sep)
	out := make([]time.Time, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, p); err == nil {
			out = append(out, t)
			continue
		}
		if t, err := time.Parse("2006-01-02T15:04:05", p); err == nil {
			out = append(out, t)
			continue
		}
		if t, err := time.Parse("2006-01-02T15:04", p); err == nil {
			out = append(out, t)
			continue
		}
		return nil, fmt.Errorf("journey: departure_times entry %q is not a valid RFC3339 timestamp", p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("journey: departure_times must contain at least one timestamp")
	}
	return out, nil
}

package journey_find_nearby_places

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
	Name         = "Find Nearby Places"
	Description  = "Find points of interest (restaurants, hotels, petrol stations etc.) around a location."
	Website      = "https://www.flomation.co"
	Icon         = "route+location-dot"
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
		Name:        "latitude",
		Type:        core.ConnectionTypeSecret,
		Label:       "Latitude",
		Placeholder: "51.5034",
		Required:    true,
	},
	{
		Name:        "longitude",
		Type:        core.ConnectionTypeString,
		Label:       "Longitude",
		Placeholder: "-0.1276",
		Required:    true,
	},
	{
		Name:        "radius_metres",
		Type:        core.ConnectionTypeString,
		Label:       "Search radius (metres)",
		Placeholder: "1500",
	},
	{
		Name:  "category",
		Type:  core.ConnectionTypeString,
		Label: "Category",
		Options: []core.ConnectionOption{
			{Name: "Any", Value: ""},
			{Name: "Restaurant", Value: "restaurant"},
			{Name: "Cafe", Value: "cafe"},
			{Name: "Petrol Station", Value: "gas_station"},
			{Name: "Hotel / Lodging", Value: "lodging"},
			{Name: "Tourist Attraction", Value: "tourist_attraction"},
			{Name: "Parking", Value: "parking"},
			{Name: "Pharmacy", Value: "pharmacy"},
			{Name: "ATM", Value: "atm"},
			{Name: "Supermarket", Value: "supermarket"},
		},
	},
	{
		Name:        "keyword",
		Type:        core.ConnectionTypeString,
		Label:       "Keyword filter",
		Placeholder: "vegetarian",
	},
	{
		Name:        "limit",
		Type:        core.ConnectionTypeString,
		Label:       "Max results",
		Placeholder: "10",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "places", Type: core.ConnectionTypeObject, Label: "Places list"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Result count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	provider, err := journey.ProviderInput(inputs)
	if err != nil {
		return nil, err
	}
	latStr, err := journey.RequiredString("latitude", inputs)
	if err != nil {
		return nil, err
	}
	lngStr, err := journey.RequiredString("longitude", inputs)
	if err != nil {
		return nil, err
	}
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return nil, fmt.Errorf("journey: latitude must be numeric, got %q", latStr)
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		return nil, fmt.Errorf("journey: longitude must be numeric, got %q", lngStr)
	}

	radius := 0
	if r := journey.OptionalString("radius_metres", inputs); r != "" {
		if v, err := strconv.Atoi(r); err == nil {
			radius = v
		}
	}
	limit := 0
	if l := journey.OptionalString("limit", inputs); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}

	places, err := provider.FindNearbyPlaces(journey.NearbyPlacesParams{
		Latitude:  lat,
		Longitude: lng,
		Radius:    radius,
		Keyword:   journey.OptionalString("keyword", inputs),
		Category:  journey.OptionalString("category", inputs),
		Limit:     limit,
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to find nearby places: %s", err.Error()),
			"success":     false,
			"error":       err.Error(),
		}, err
	}

	summary := fmt.Sprintf("Found %d nearby place(s).", len(places))
	if len(places) > 0 {
		summary = fmt.Sprintf("Found %d place(s): top result %q (%.0fm).", len(places), places[0].Name, places[0].DistanceM)
	}

	return map[string]interface{}{
		"tool_result": summary,
		"places":      json.RawMessage(mustJSON(places)),
		"count":       fmt.Sprintf("%d", len(places)),
		"success":     true,
		"error":       "",
	}, nil
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("[]")
	}
	return b
}

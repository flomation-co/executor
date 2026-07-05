package journey_reverse_geocode

import (
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Reverse Geocode"
	Description  = "Convert latitude and longitude into a human-readable address."
	Website      = "https://www.flomation.co"
	Icon         = "route+map"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "address", Type: core.ConnectionTypeString, Label: "Full address"},
	{Name: "street", Type: core.ConnectionTypeString, Label: "Street"},
	{Name: "city", Type: core.ConnectionTypeString, Label: "City"},
	{Name: "country", Type: core.ConnectionTypeString, Label: "Country"},
	{Name: "postcode", Type: core.ConnectionTypeString, Label: "Postcode"},
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

	result, err := provider.ReverseGeocode(lat, lng)
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to reverse geocode (%f, %f): %s", lat, lng, err.Error()),
			"success":     false,
			"error":       err.Error(),
		}, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("(%f, %f) → %s", lat, lng, result.Address),
		"address":     result.Address,
		"street":      result.Street,
		"city":        result.City,
		"country":     result.Country,
		"postcode":    result.Postcode,
		"success":     true,
		"error":       "",
	}, nil
}

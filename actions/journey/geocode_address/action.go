package journey_geocode_address

import (
	"fmt"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Geocode Address"
	Description  = "Convert a textual address into latitude and longitude coordinates."
	Website      = "https://www.flomation.co"
	Icon         = "route+magnifying-glass"
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
		Name:        "address",
		Type:        core.ConnectionTypeSecret,
		Label:       "Address",
		Placeholder: "10 Downing Street, London",
		Required:    true,
	},
	{
		Name:        "region",
		Type:        core.ConnectionTypeString,
		Label:       "Region (country code)",
		Placeholder: "gb",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "latitude", Type: core.ConnectionTypeString, Label: "Latitude"},
	{Name: "longitude", Type: core.ConnectionTypeString, Label: "Longitude"},
	{Name: "formatted_address", Type: core.ConnectionTypeString, Label: "Formatted Address"},
	{Name: "confidence", Type: core.ConnectionTypeString, Label: "Confidence"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	provider, err := journey.ProviderInput(inputs)
	if err != nil {
		return nil, err
	}
	address, err := journey.RequiredString("address", inputs)
	if err != nil {
		return nil, err
	}
	region := journey.OptionalString("region", inputs)

	result, err := provider.Geocode(address, region)
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to geocode %q: %s", address, err.Error()),
			"success":     false,
			"error":       err.Error(),
		}, err
	}

	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("%s → (%f, %f)", result.FormattedAddress, result.Latitude, result.Longitude),
		"latitude":          fmt.Sprintf("%f", result.Latitude),
		"longitude":         fmt.Sprintf("%f", result.Longitude),
		"formatted_address": result.FormattedAddress,
		"confidence":        result.Confidence,
		"success":           true,
		"error":             "",
	}, nil
}

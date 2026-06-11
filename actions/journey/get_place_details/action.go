package journey_get_place_details

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Place Details"
	Description  = "Fetch opening hours, phone, website and rating details for a place ID."
	Website      = "https://www.flomation.co"
	Icon         = "route+circle-info"
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
		Name:        "place_id",
		Type:        core.ConnectionTypeString,
		Label:       "Place ID (provider-scoped)",
		Placeholder: "${parent.places.0.place_id}",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Place name"},
	{Name: "address", Type: core.ConnectionTypeString, Label: "Full address"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone number"},
	{Name: "website", Type: core.ConnectionTypeString, Label: "Website"},
	{Name: "rating", Type: core.ConnectionTypeString, Label: "Rating (0-5)"},
	{Name: "opening_hours", Type: core.ConnectionTypeObject, Label: "Opening hours per day"},
	{Name: "google_url", Type: core.ConnectionTypeString, Label: "Google Maps URL"},
	{Name: "details", Type: core.ConnectionTypeObject, Label: "Full place details JSON"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	provider, err := journey.ProviderInput(inputs)
	if err != nil {
		return nil, err
	}
	placeID, err := journey.RequiredString("place_id", inputs)
	if err != nil {
		return nil, err
	}

	details, err := provider.GetPlaceDetails(placeID)
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to fetch place details for %s: %s", placeID, err.Error()),
			"success":     false,
			"error":       err.Error(),
		}, err
	}

	ratingStr := ""
	if details.Rating > 0 {
		ratingStr = fmt.Sprintf("%.1f", details.Rating)
	}
	summary := fmt.Sprintf("%s — %s", details.Name, details.Address)
	if ratingStr != "" {
		summary += fmt.Sprintf(" (★ %s)", ratingStr)
	}
	if len(details.OpeningHours) > 0 {
		summary += " — opening hours available"
	}

	detailsJSON, _ := json.Marshal(details)

	return map[string]interface{}{
		"tool_result":   summary,
		"name":          details.Name,
		"address":       details.Address,
		"phone":         details.Phone,
		"website":       details.Website,
		"rating":        ratingStr,
		"opening_hours": details.OpeningHours,
		"google_url":    details.GoogleURL,
		"details":       json.RawMessage(detailsJSON),
		"success":       true,
		"error":         "",
	}, nil
}


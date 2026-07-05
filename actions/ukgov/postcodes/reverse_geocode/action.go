// Package ukgov_postcodes_reverse_geocode finds the nearest UK postcodes to a
// latitude/longitude via the free postcodes.io API.
package ukgov_postcodes_reverse_geocode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/postcodes"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Reverse Geocode"
	Description  = "Find the nearest UK postcodes to a latitude/longitude (postcodes.io)"
	Website      = "https://www.flomation.co"
	Icon         = "map+location-arrow"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "latitude", Type: core.ConnectionTypeString, Label: "Latitude", Placeholder: "51.501009", Required: true},
	{Name: "longitude", Type: core.ConnectionTypeString, Label: "Longitude", Placeholder: "-0.141588", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Nearby Postcodes"},
	{Name: "nearest", Type: core.ConnectionTypeString, Label: "Nearest Postcode"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type reverseResponse struct {
	Status int                  `json:"status"`
	Result []postcodes.Postcode `json:"result"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	lat, err := ukgov_common.RequiredString("latitude", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A latitude is required.")
	}
	lng, err := ukgov_common.RequiredString("longitude", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A longitude is required.")
	}

	q := url.Values{}
	q.Set("lat", lat)
	q.Set("lon", lng)

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := postcodes.Get(ctx, "/postcodes", q)
	if err != nil {
		return ukgov_common.ErrResult("Reverse geocode request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Reverse geocode returned status %d", status)
	}

	var parsed reverseResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse reverse geocode response: %v", err)
	}

	if len(parsed.Result) == 0 {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("No UK postcodes found near %s, %s.", lat, lng),
			"results":     []postcodes.Postcode{},
			"nearest":     "",
			"count":       0,
			"success":     true,
			"error":       "",
		}, nil
	}

	nearest := parsed.Result[0]
	summary := fmt.Sprintf("Nearest postcode to %s, %s is %s (%s, %s). %d postcode(s) nearby.",
		lat, lng, nearest.Postcode, nearest.AdminDistrict, nearest.Region, len(parsed.Result))

	return map[string]interface{}{
		"tool_result": summary,
		"results":     parsed.Result,
		"nearest":     nearest.Postcode,
		"count":       len(parsed.Result),
		"success":     true,
		"error":       "",
	}, nil
}

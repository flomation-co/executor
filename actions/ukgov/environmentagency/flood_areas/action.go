// Package ukgov_environmentagency_flood_areas lists Environment Agency flood
// areas near a coordinate. No authentication required.
package ukgov_environmentagency_flood_areas

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/environmentagency"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Flood Areas"
	Description  = "List UK flood warning/alert areas near a latitude/longitude (Environment Agency)"
	Website      = "https://www.flomation.co"
	Icon         = "leaf+map"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "latitude", Type: core.ConnectionTypeString, Label: "Latitude", Placeholder: "52.7", Required: true},
	{Name: "longitude", Type: core.ConnectionTypeString, Label: "Longitude", Placeholder: "-2.75", Required: true},
	{Name: "distance_km", Type: core.ConnectionTypeInteger, Label: "Radius (km)", Placeholder: "10"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "areas", Type: core.ConnectionTypeObject, Label: "Flood Areas"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type floodArea struct {
	Description string  `json:"description"`
	County      string  `json:"county"`
	Label       string  `json:"label"`
	Notation    string  `json:"notation"`
	RiverOrSea  string  `json:"riverOrSea"`
	Lat         float64 `json:"lat"`
	Long        float64 `json:"long"`
	FwdCode     string  `json:"fwdCode"`
}

type areasResponse struct {
	Items []floodArea `json:"items"`
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
	dist := ukgov_common.OptionalInt("distance_km", inputs, 10)
	if dist <= 0 {
		dist = 10
	}

	q := url.Values{}
	q.Set("lat", lat)
	q.Set("long", lng)
	q.Set("dist", fmt.Sprintf("%d", dist))

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := environmentagency.Get(ctx, "/id/floodAreas", q)
	if err != nil {
		return ukgov_common.ErrResult("Environment Agency request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Environment Agency returned status %d", status)
	}

	var parsed areasResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse Environment Agency response: %v", err)
	}

	summary := fmt.Sprintf("No flood areas found within %dkm of %s, %s.", dist, lat, lng)
	if len(parsed.Items) > 0 {
		summary = fmt.Sprintf("Found %d flood area(s) within %dkm of %s, %s. Nearest: %s.",
			len(parsed.Items), dist, lat, lng, parsed.Items[0].Label)
	}

	return map[string]interface{}{
		"tool_result": summary,
		"areas":       parsed.Items,
		"count":       len(parsed.Items),
		"success":     true,
		"error":       "",
	}, nil
}

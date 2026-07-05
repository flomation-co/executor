// Package ukgov_postcodes_postcode_lookup resolves a UK postcode to its
// location and administrative areas via the free postcodes.io API.
package ukgov_postcodes_postcode_lookup

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
	Name         = "Postcode Lookup"
	Description  = "Resolve a UK postcode to coordinates, region and administrative areas (postcodes.io)"
	Website      = "https://www.flomation.co"
	Icon         = "location-dot+magnifying-glass"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "postcode", Type: core.ConnectionTypeString, Label: "Postcode", Placeholder: "SW1A 1AA", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Postcode Details"},
	{Name: "latitude", Type: core.ConnectionTypeString, Label: "Latitude"},
	{Name: "longitude", Type: core.ConnectionTypeString, Label: "Longitude"},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type lookupResponse struct {
	Status int                 `json:"status"`
	Result *postcodes.Postcode `json:"result"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	pc, err := ukgov_common.RequiredString("postcode", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A postcode is required.")
	}

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := postcodes.Get(ctx, "/postcodes/"+url.PathEscape(pc), nil)
	if err != nil {
		return ukgov_common.ErrResult("Postcode lookup request failed: %v", err)
	}
	if status == http.StatusNotFound {
		return ukgov_common.ErrResult("%q is not a recognised UK postcode.", pc)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Postcode lookup returned status %d", status)
	}

	var parsed lookupResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse postcode response: %v", err)
	}
	if parsed.Result == nil {
		return ukgov_common.ErrResult("%q is not a recognised UK postcode.", pc)
	}

	r := parsed.Result
	summary := fmt.Sprintf("%s is in %s, %s (%s). Coordinates: %.5f, %.5f.",
		r.Postcode, r.AdminDistrict, r.Region, r.Country, r.Latitude, r.Longitude)

	return map[string]interface{}{
		"tool_result": summary,
		"result":      r,
		"latitude":    fmt.Sprintf("%.6f", r.Latitude),
		"longitude":   fmt.Sprintf("%.6f", r.Longitude),
		"region":      r.Region,
		"success":     true,
		"error":       "",
	}, nil
}

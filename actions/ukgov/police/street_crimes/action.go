// Package ukgov_police_street_crimes returns street-level crimes reported near
// a coordinate for a given month, using the free data.police.uk API.
package ukgov_police_street_crimes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/police"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Street-Level Crimes"
	Description  = "List UK street-level crimes near a latitude/longitude for a given month (Police UK)"
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+map"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "latitude", Type: core.ConnectionTypeString, Label: "Latitude", Placeholder: "52.629729", Required: true},
	{Name: "longitude", Type: core.ConnectionTypeString, Label: "Longitude", Placeholder: "-1.131592", Required: true},
	{Name: "date", Type: core.ConnectionTypeString, Label: "Month (YYYY-MM, optional)", Placeholder: "2024-01"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "crimes", Type: core.ConnectionTypeObject, Label: "Crimes"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
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
	date := ukgov_common.OptionalString("date", inputs)

	q := url.Values{}
	q.Set("lat", lat)
	q.Set("lng", lng)
	if date != "" {
		q.Set("date", date)
	}

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := police.Get(ctx, "/crimes-street/all-crime", q)
	if err != nil {
		return ukgov_common.ErrResult("Police UK request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Police UK returned status %d (check the coordinate and month)", status)
	}

	var crimes []police.Crime
	if err := json.Unmarshal(body, &crimes); err != nil {
		return ukgov_common.ErrResult("Failed to parse Police UK response: %v", err)
	}

	return map[string]interface{}{
		"tool_result": summarise(crimes, lat, lng),
		"crimes":      crimes,
		"count":       len(crimes),
		"success":     true,
		"error":       "",
	}, nil
}

// summarise builds a concise, AI-readable summary grouping crimes by category.
func summarise(crimes []police.Crime, lat, lng string) string {
	if len(crimes) == 0 {
		return fmt.Sprintf("No street crimes found near %s, %s for that period.", lat, lng)
	}
	counts := make(map[string]int, len(crimes))
	month := crimes[0].Month
	for _, c := range crimes {
		counts[c.Category]++
	}
	return fmt.Sprintf("Found %d street crime(s) near %s, %s in %s. Top categories: %s.",
		len(crimes), lat, lng, month, police.TopCounts(counts, 5))
}

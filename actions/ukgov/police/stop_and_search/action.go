// Package ukgov_police_stop_and_search returns stop-and-search records near a
// coordinate for a given month, using the free data.police.uk API.
package ukgov_police_stop_and_search

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
	Name         = "Stop and Search"
	Description  = "List UK stop-and-search records near a latitude/longitude for a given month (Police UK)"
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+magnifying-glass"
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
	{Name: "searches", Type: core.ConnectionTypeObject, Label: "Stop and Search Records"},
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

	status, body, err := police.Get(ctx, "/stops-street", q)
	if err != nil {
		return ukgov_common.ErrResult("Police UK request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Police UK returned status %d (check the coordinate and month)", status)
	}

	var stops []police.Stop
	if err := json.Unmarshal(body, &stops); err != nil {
		return ukgov_common.ErrResult("Failed to parse Police UK response: %v", err)
	}

	return map[string]interface{}{
		"tool_result": summarise(stops, lat, lng),
		"searches":    stops,
		"count":       len(stops),
		"success":     true,
		"error":       "",
	}, nil
}

// summarise builds a concise summary grouping stops by object of search.
func summarise(stops []police.Stop, lat, lng string) string {
	if len(stops) == 0 {
		return fmt.Sprintf("No stop-and-search records found near %s, %s for that period.", lat, lng)
	}
	counts := make(map[string]int, len(stops))
	for _, s := range stops {
		obj := s.ObjectOfSearch
		if obj == "" {
			obj = "unspecified"
		}
		counts[obj]++
	}
	return fmt.Sprintf("Found %d stop-and-search record(s) near %s, %s. By object of search: %s.",
		len(stops), lat, lng, police.TopCounts(counts, 5))
}

// Package ukgov_environmentagency_station_readings returns the latest measured
// values (river level, flow, rainfall) for an Environment Agency monitoring
// station. No authentication required.
package ukgov_environmentagency_station_readings

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/environmentagency"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Station Readings"
	Description  = "Get the latest river level, flow or rainfall readings for a monitoring station (Environment Agency)"
	Website      = "https://www.flomation.co"
	Icon         = "leaf+gauge"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "station_reference", Type: core.ConnectionTypeString, Label: "Station Reference", Placeholder: "2001", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "measures", Type: core.ConnectionTypeObject, Label: "Measures"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type latestReading struct {
	Value    *float64 `json:"value"`
	DateTime string   `json:"dateTime"`
}

type measure struct {
	Parameter     string         `json:"parameter"`
	ParameterName string         `json:"parameterName"`
	Qualifier     string         `json:"qualifier"`
	UnitName      string         `json:"unitName"`
	LatestReading *latestReading `json:"latestReading"`
}

type measuresResponse struct {
	Items []measure `json:"items"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ref, err := ukgov_common.RequiredString("station_reference", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A station reference is required.")
	}

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	// `latest` is a valueless flag, so it is appended to the path directly
	// rather than encoded as an empty-valued query parameter.
	path := "/id/measures?stationReference=" + url.QueryEscape(ref) + "&latest"
	status, body, err := environmentagency.Get(ctx, path, nil)
	if err != nil {
		return ukgov_common.ErrResult("Environment Agency request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Environment Agency returned status %d", status)
	}

	var parsed measuresResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse Environment Agency response: %v", err)
	}

	if len(parsed.Items) == 0 {
		return ukgov_common.ErrResult("No readings found for station %q.", ref)
	}

	return map[string]interface{}{
		"tool_result": summarise(parsed.Items, ref),
		"measures":    parsed.Items,
		"count":       len(parsed.Items),
		"success":     true,
		"error":       "",
	}, nil
}

// summarise lists each measure's latest value, skipping any with no reading.
func summarise(measures []measure, ref string) string {
	parts := make([]string, 0, len(measures))
	for _, m := range measures {
		if m.LatestReading == nil || m.LatestReading.Value == nil {
			continue
		}
		label := m.ParameterName
		if m.Qualifier != "" {
			label += " (" + m.Qualifier + ")"
		}
		parts = append(parts, fmt.Sprintf("%s %g %s", label, *m.LatestReading.Value, m.UnitName))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Station %q has measures but no current readings.", ref)
	}
	return fmt.Sprintf("Station %q latest readings: %s.", ref, strings.Join(parts, "; "))
}

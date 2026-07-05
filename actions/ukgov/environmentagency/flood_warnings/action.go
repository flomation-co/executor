// Package ukgov_environmentagency_flood_warnings returns current UK flood
// warnings and alerts from the Environment Agency. No authentication required.
package ukgov_environmentagency_flood_warnings

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
	Name         = "Flood Warnings"
	Description  = "List current UK flood warnings and alerts, optionally by county (Environment Agency)"
	Website      = "https://www.flomation.co"
	Icon         = "water+triangle-exclamation"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "county", Type: core.ConnectionTypeString, Label: "County (optional)", Placeholder: "Shropshire"},
	{Name: "min_severity", Type: core.ConnectionTypeInteger, Label: "Minimum Severity (1=Severe … 4=No longer in force)", Placeholder: "3"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "warnings", Type: core.ConnectionTypeObject, Label: "Flood Warnings"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type floodArea struct {
	County     string `json:"county"`
	RiverOrSea string `json:"riverOrSea"`
	Notation   string `json:"notation"`
}

type flood struct {
	Description   string    `json:"description"`
	Severity      string    `json:"severity"`
	SeverityLevel int       `json:"severityLevel"`
	FloodAreaID   string    `json:"floodAreaID"`
	Message       string    `json:"message"`
	TimeRaised    string    `json:"timeRaised"`
	EaAreaName    string    `json:"eaAreaName"`
	IsTidal       bool      `json:"isTidal"`
	FloodArea     floodArea `json:"floodArea"`
}

type floodsResponse struct {
	Items []flood `json:"items"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	county := ukgov_common.OptionalString("county", inputs)
	minSeverity := ukgov_common.OptionalInt("min_severity", inputs, 0)

	q := url.Values{}
	if county != "" {
		q.Set("county", county)
	}
	if minSeverity >= 1 && minSeverity <= 4 {
		q.Set("min-severity", fmt.Sprintf("%d", minSeverity))
	}

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := environmentagency.Get(ctx, "/id/floods", q)
	if err != nil {
		return ukgov_common.ErrResult("Environment Agency request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Environment Agency returned status %d", status)
	}

	var parsed floodsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse Environment Agency response: %v", err)
	}

	return map[string]interface{}{
		"tool_result": summarise(parsed.Items, county),
		"warnings":    parsed.Items,
		"count":       len(parsed.Items),
		"success":     true,
		"error":       "",
	}, nil
}

// summarise groups warnings by severity label and names the most severe area.
func summarise(floods []flood, county string) string {
	scope := "across the UK"
	if county != "" {
		scope = "in " + county
	}
	if len(floods) == 0 {
		return fmt.Sprintf("No active flood warnings %s.", scope)
	}

	counts := make(map[string]int, len(floods))
	mostSevere := floods[0]
	for _, f := range floods {
		label := f.Severity
		if label == "" {
			label = "Unknown"
		}
		counts[label]++
		if f.SeverityLevel != 0 && (mostSevere.SeverityLevel == 0 || f.SeverityLevel < mostSevere.SeverityLevel) {
			mostSevere = f
		}
	}

	parts := make([]string, 0, len(counts))
	// Iterate severity levels high→low for a stable, meaningful order.
	for _, label := range []string{"Severe Flood Warning", "Flood Warning", "Flood Alert", "Warning no Longer in Force"} {
		if n, ok := counts[label]; ok {
			parts = append(parts, fmt.Sprintf("%s (%d)", label, n))
		}
	}

	return fmt.Sprintf("%d active flood warning(s) %s. By severity: %s. Most severe: %s.",
		len(floods), scope, joinOr(parts, "mixed"), mostSevere.Description)
}

func joinOr(parts []string, fallback string) string {
	if len(parts) == 0 {
		return fallback
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += ", " + p
	}
	return out
}

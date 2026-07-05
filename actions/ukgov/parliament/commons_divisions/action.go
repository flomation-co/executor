// Package ukgov_parliament_commons_divisions searches recorded House of Commons
// division (vote) results. No authentication required.
package ukgov_parliament_commons_divisions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Commons Divisions"
	Description  = "Search recorded House of Commons division (vote) results (UK Parliament)"
	Website      = "https://www.flomation.co"
	Icon         = "landmark+check"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var baseURL = "https://commonsvotes-api.parliament.uk"

var Inputs = [...]core.Connection{
	{Name: "query", Type: core.ConnectionTypeString, Label: "Search Term (optional)", Placeholder: "e.g. budget"},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results (1-25)", Placeholder: "20"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "divisions", Type: core.ConnectionTypeObject, Label: "Divisions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// division uses PascalCase JSON keys (this API differs from the other
// Parliament APIs, which are camelCase).
type division struct {
	DivisionId int    `json:"DivisionId"`
	Date       string `json:"Date"`
	Number     int    `json:"Number"`
	Title      string `json:"Title"`
	AyeCount   int    `json:"AyeCount"`
	NoCount    int    `json:"NoCount"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	query := ukgov_common.OptionalString("query", inputs)
	maxResults := ukgov_common.OptionalInt("max_results", inputs, 20)
	if maxResults <= 0 {
		maxResults = 20
	}
	if maxResults > 25 {
		maxResults = 25
	}

	q := url.Values{}
	if query != "" {
		q.Set("searchTerm", query)
	}
	q.Set("take", fmt.Sprintf("%d", maxResults))
	// The `.json` path segment is required by this API.
	endpoint := baseURL + "/data/divisions.json/search?" + q.Encode()

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := ukgov_common.Fetch(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ukgov_common.ErrResult("UK Parliament request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("UK Parliament returned status %d", status)
	}

	// This endpoint returns a bare JSON array (no wrapper).
	var divisions []division
	if err := json.Unmarshal(body, &divisions); err != nil {
		return ukgov_common.ErrResult("Failed to parse UK Parliament response: %v", err)
	}

	return map[string]interface{}{
		"tool_result": summarise(divisions, query),
		"divisions":   divisions,
		"count":       len(divisions),
		"success":     true,
		"error":       "",
	}, nil
}

func summarise(divisions []division, query string) string {
	scope := ""
	if query != "" {
		scope = fmt.Sprintf(" matching %q", query)
	}
	if len(divisions) == 0 {
		return fmt.Sprintf("No Commons divisions found%s.", scope)
	}
	limit := len(divisions)
	if limit > 5 {
		limit = 5
	}
	parts := make([]string, 0, limit)
	for _, d := range divisions[:limit] {
		date := d.Date
		if len(date) >= 10 {
			date = date[:10]
		}
		parts = append(parts, fmt.Sprintf("%s — Ayes %d, Noes %d (%s)", d.Title, d.AyeCount, d.NoCount, date))
	}
	return fmt.Sprintf("Found %d Commons division(s)%s. Recent: %s.", len(divisions), scope, strings.Join(parts, "; "))
}

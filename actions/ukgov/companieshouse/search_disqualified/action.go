// Package ukgov_companieshouse_search_disqualified searches the UK Companies
// House register of disqualified company directors by name.
package ukgov_companieshouse_search_disqualified

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/companieshouse"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Search Disqualified Officers"
	Description  = "Search the UK register of disqualified company directors by name (Companies House)"
	Website      = "https://www.flomation.co"
	Icon         = "building+user-slash"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Companies House API Key", Placeholder: "${secrets.COMPANIES_HOUSE_KEY}", Required: true},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Officer Name", Placeholder: "e.g. John Smith", Required: true},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results (1-100)", Placeholder: "20"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "officers", Type: core.ConnectionTypeObject, Label: "Disqualified Officers"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total Matches"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// officer note: date_of_birth is a plain date string here (the officer search
// endpoint uses a {month, year} object instead).
type officer struct {
	Title          string `json:"title"`
	AddressSnippet string `json:"address_snippet"`
	Description    string `json:"description"`
	DateOfBirth    string `json:"date_of_birth"`
}

type searchResponse struct {
	Items        []officer `json:"items"`
	TotalResults int       `json:"total_results"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := ukgov_common.RequiredString("api_key", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A Companies House API key is required.")
	}
	query, err := ukgov_common.RequiredString("query", inputs)
	if err != nil {
		return ukgov_common.ErrResult("An officer name to search for is required.")
	}
	maxResults := ukgov_common.OptionalInt("max_results", inputs, 20)
	if maxResults <= 0 {
		maxResults = 20
	}
	if maxResults > 100 {
		maxResults = 100
	}

	q := url.Values{}
	q.Set("q", query)
	q.Set("items_per_page", fmt.Sprintf("%d", maxResults))

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := companieshouse.Get(ctx, apiKey, "/search/disqualified-officers", q)
	if err != nil {
		return ukgov_common.ErrResult("Companies House request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("%s", companieshouse.StatusMessage(status))
	}

	var parsed searchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse Companies House response: %v", err)
	}

	summary := fmt.Sprintf("No disqualified officers found matching %q.", query)
	if len(parsed.Items) > 0 {
		limit := len(parsed.Items)
		if limit > 5 {
			limit = 5
		}
		parts := make([]string, 0, limit)
		for _, o := range parsed.Items[:limit] {
			parts = append(parts, o.Title)
		}
		summary = fmt.Sprintf("Found %d disqualified officer(s) matching %q. Top: %s.", parsed.TotalResults, query, strings.Join(parts, "; "))
	}

	return map[string]interface{}{
		"tool_result": summary,
		"officers":    parsed.Items,
		"count":       len(parsed.Items),
		"total":       parsed.TotalResults,
		"success":     true,
		"error":       "",
	}, nil
}

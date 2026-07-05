// Package ukgov_companieshouse_search_companies searches the UK Companies House
// register by name or number.
package ukgov_companieshouse_search_companies

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
	Name         = "Search Companies"
	Description  = "Search the UK Companies House register by company name or number (Companies House)"
	Website      = "https://www.flomation.co"
	Icon         = "building+magnifying-glass"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Companies House API Key", Placeholder: "${secrets.COMPANIES_HOUSE_KEY}", Required: true},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Search Query", Placeholder: "e.g. Flomation Ltd", Required: true},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results (1-100)", Placeholder: "20"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "companies", Type: core.ConnectionTypeObject, Label: "Companies"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total Matches"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type company struct {
	CompanyNumber  string `json:"company_number"`
	Title          string `json:"title"`
	CompanyStatus  string `json:"company_status"`
	CompanyType    string `json:"company_type"`
	AddressSnippet string `json:"address_snippet"`
	DateOfCreation string `json:"date_of_creation"`
}

type searchResponse struct {
	Items        []company `json:"items"`
	TotalResults int       `json:"total_results"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := ukgov_common.RequiredString("api_key", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A Companies House API key is required.")
	}
	query, err := ukgov_common.RequiredString("query", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A search query is required.")
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

	status, body, err := companieshouse.Get(ctx, apiKey, "/search/companies", q)
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

	return map[string]interface{}{
		"tool_result": summarise(parsed.Items, parsed.TotalResults, query),
		"companies":   parsed.Items,
		"count":       len(parsed.Items),
		"total":       parsed.TotalResults,
		"success":     true,
		"error":       "",
	}, nil
}

func summarise(companies []company, total int, query string) string {
	if len(companies) == 0 {
		return fmt.Sprintf("No companies found matching %q.", query)
	}
	limit := len(companies)
	if limit > 5 {
		limit = 5
	}
	parts := make([]string, 0, limit)
	for _, c := range companies[:limit] {
		parts = append(parts, fmt.Sprintf("%s (%s, %s)", c.Title, c.CompanyNumber, c.CompanyStatus))
	}
	return fmt.Sprintf("Found %d companies matching %q. Top: %s.", total, query, strings.Join(parts, "; "))
}

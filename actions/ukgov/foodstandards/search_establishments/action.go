// Package ukgov_foodstandards_search_establishments searches the UK Food
// Standards Agency's Food Hygiene Rating Scheme (FHRS) for establishments by
// business name and/or address. The FHRS API requires no authentication.
package ukgov_foodstandards_search_establishments

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
	Name         = "Search Food Establishments"
	Description  = "Search UK food hygiene ratings by business name or address (Food Standards Agency)"
	Website      = "https://www.flomation.co"
	Icon         = "utensils+magnifying-glass"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

// baseURL is the FHRS API root. It is a package variable (not a const) so tests
// can redirect it to a local mock server.
var baseURL = "https://api.ratings.food.gov.uk"

// apiVersion is the mandatory FHRS API version header value.
const apiVersion = "2"

var Inputs = [...]core.Connection{
	{
		Name:        "name",
		Type:        core.ConnectionTypeString,
		Label:       "Business Name",
		Placeholder: "e.g. The Ivy",
	},
	{
		Name:        "address",
		Type:        core.ConnectionTypeString,
		Label:       "Address or Postcode",
		Placeholder: "e.g. SW1A 1AA",
	},
	{
		Name:        "max_results",
		Type:        core.ConnectionTypeInteger,
		Label:       "Maximum Results (1-50)",
		Placeholder: "10",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "establishments", Type: core.ConnectionTypeObject, Label: "Establishments"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total Matches"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type geocode struct {
	Longitude string `json:"longitude"`
	Latitude  string `json:"latitude"`
}

type establishment struct {
	FHRSID             int64   `json:"FHRSID"`
	BusinessName       string  `json:"BusinessName"`
	BusinessType       string  `json:"BusinessType"`
	AddressLine1       string  `json:"AddressLine1"`
	AddressLine2       string  `json:"AddressLine2"`
	AddressLine3       string  `json:"AddressLine3"`
	AddressLine4       string  `json:"AddressLine4"`
	PostCode           string  `json:"PostCode"`
	RatingValue        string  `json:"RatingValue"`
	RatingDate         string  `json:"RatingDate"`
	LocalAuthorityName string  `json:"LocalAuthorityName"`
	Geocode            geocode `json:"geocode"`
}

type searchResponse struct {
	Establishments []establishment `json:"establishments"`
	Meta           struct {
		TotalCount int `json:"totalCount"`
	} `json:"meta"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	name := ukgov_common.OptionalString("name", inputs)
	address := ukgov_common.OptionalString("address", inputs)
	maxResults := ukgov_common.OptionalInt("max_results", inputs, 10)

	if strings.TrimSpace(name) == "" && strings.TrimSpace(address) == "" {
		return ukgov_common.ErrResult("Provide a business name or address to search for.")
	}
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 50 {
		maxResults = 50
	}

	q := url.Values{}
	if name != "" {
		q.Set("name", name)
	}
	if address != "" {
		q.Set("address", address)
	}
	q.Set("pageNumber", "1")
	q.Set("pageSize", fmt.Sprintf("%d", maxResults))
	endpoint := fmt.Sprintf("%s/Establishments?%s", baseURL, q.Encode())

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := ukgov_common.Fetch(ctx, http.MethodGet, endpoint, map[string]string{
		"x-api-version": apiVersion,
	})
	if err != nil {
		return ukgov_common.ErrResult("Food Standards Agency request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Food Standards Agency returned status %d", status)
	}

	var parsed searchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse Food Standards Agency response: %v", err)
	}

	displayQuery := strings.TrimSpace(strings.Join(nonEmpty(name, address), " "))
	summary := summarise(parsed.Establishments, parsed.Meta.TotalCount, displayQuery)

	return map[string]interface{}{
		"tool_result":    summary,
		"establishments": parsed.Establishments,
		"count":          len(parsed.Establishments),
		"total":          parsed.Meta.TotalCount,
		"success":        true,
		"error":          "",
	}, nil
}

// nonEmpty returns the non-empty members of vals, preserving order.
func nonEmpty(vals ...string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

// summarise builds a concise, AI-readable one-line summary of the results,
// listing up to the first five establishments with their hygiene rating.
func summarise(estabs []establishment, total int, query string) string {
	if len(estabs) == 0 {
		if query != "" {
			return fmt.Sprintf("No food establishments found matching %q.", query)
		}
		return "No food establishments found."
	}

	var b strings.Builder
	if query != "" {
		fmt.Fprintf(&b, "Found %d establishment(s) matching %q.", total, query)
	} else {
		fmt.Fprintf(&b, "Found %d establishment(s).", total)
	}

	limit := len(estabs)
	if limit > 5 {
		limit = 5
	}
	parts := make([]string, 0, limit)
	for _, e := range estabs[:limit] {
		rating := strings.TrimSpace(e.RatingValue)
		if rating == "" {
			rating = "unrated"
		}
		loc := strings.TrimSpace(e.PostCode)
		if loc == "" {
			loc = strings.TrimSpace(e.LocalAuthorityName)
		}
		parts = append(parts, fmt.Sprintf("%s — hygiene rating %s (%s)", e.BusinessName, rating, loc))
	}
	fmt.Fprintf(&b, " Top: %s.", strings.Join(parts, "; "))
	return b.String()
}

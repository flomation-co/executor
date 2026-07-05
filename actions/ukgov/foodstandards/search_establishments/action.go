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
	"flomation.app/automate/executor/actions/ukgov/foodstandards"
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

type searchResponse struct {
	Establishments []foodstandards.Establishment `json:"establishments"`
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

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := foodstandards.Get(ctx, "/Establishments", q)
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
func summarise(estabs []foodstandards.Establishment, total int, query string) string {
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

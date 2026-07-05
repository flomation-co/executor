// Package ukgov_parliament_search_members searches for members of the UK
// Parliament (MPs and Lords) by name. No authentication required.
package ukgov_parliament_search_members

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
	Name         = "Search Members"
	Description  = "Search UK Parliament members (MPs and Lords) by name (UK Parliament)"
	Website      = "https://www.flomation.co"
	Icon         = "landmark+magnifying-glass"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var baseURL = "https://members-api.parliament.uk"

var Inputs = [...]core.Connection{
	{Name: "name", Type: core.ConnectionTypeString, Label: "Member Name", Placeholder: "e.g. Keir Starmer", Required: true},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results (1-20)", Placeholder: "20"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "members", Type: core.ConnectionTypeObject, Label: "Members"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total Matches"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type party struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
}

type houseMembership struct {
	MembershipFrom string `json:"membershipFrom"`
	House          int    `json:"house"` // 1 = Commons, 2 = Lords
}

type memberValue struct {
	ID                    int             `json:"id"`
	NameDisplayAs         string          `json:"nameDisplayAs"`
	Gender                string          `json:"gender"`
	LatestParty           party           `json:"latestParty"`
	LatestHouseMembership houseMembership `json:"latestHouseMembership"`
}

type memberItem struct {
	Value memberValue `json:"value"`
}

type searchResponse struct {
	Items        []memberItem `json:"items"`
	TotalResults int          `json:"totalResults"`
}

func houseName(h int) string {
	switch h {
	case 1:
		return "Commons"
	case 2:
		return "Lords"
	default:
		return ""
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	name, err := ukgov_common.RequiredString("name", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A member name to search for is required.")
	}
	maxResults := ukgov_common.OptionalInt("max_results", inputs, 20)
	if maxResults <= 0 {
		maxResults = 20
	}
	if maxResults > 20 {
		maxResults = 20
	}

	q := url.Values{}
	q.Set("Name", name)
	q.Set("take", fmt.Sprintf("%d", maxResults))
	endpoint := baseURL + "/api/Members/Search?" + q.Encode()

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

	var parsed searchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse UK Parliament response: %v", err)
	}

	return map[string]interface{}{
		"tool_result": summarise(parsed.Items, parsed.TotalResults, name),
		"members":     parsed.Items,
		"count":       len(parsed.Items),
		"total":       parsed.TotalResults,
		"success":     true,
		"error":       "",
	}, nil
}

func summarise(items []memberItem, total int, name string) string {
	if len(items) == 0 {
		return fmt.Sprintf("No members found matching %q.", name)
	}
	limit := len(items)
	if limit > 5 {
		limit = 5
	}
	parts := make([]string, 0, limit)
	for _, it := range items[:limit] {
		v := it.Value
		where := v.LatestHouseMembership.MembershipFrom
		if h := houseName(v.LatestHouseMembership.House); h != "" {
			where = fmt.Sprintf("%s, %s", where, h)
		}
		parts = append(parts, fmt.Sprintf("%s (%s — %s)", v.NameDisplayAs, v.LatestParty.Name, where))
	}
	return fmt.Sprintf("Found %d member(s) matching %q. Top: %s.", total, name, strings.Join(parts, "; "))
}

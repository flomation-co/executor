// Package ukgov_parliament_search_bills searches UK Parliament bills by
// keyword. No authentication required.
package ukgov_parliament_search_bills

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
	Name         = "Search Bills"
	Description  = "Search UK Parliament bills by keyword, with their current stage (UK Parliament)"
	Website      = "https://www.flomation.co"
	Icon         = "landmark+book"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var baseURL = "https://bills-api.parliament.uk"

var Inputs = [...]core.Connection{
	{Name: "query", Type: core.ConnectionTypeString, Label: "Search Term", Placeholder: "e.g. finance", Required: true},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results (1-20)", Placeholder: "20"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "bills", Type: core.ConnectionTypeObject, Label: "Bills"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total Matches"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type currentStage struct {
	Description string `json:"description"`
	House       string `json:"house"`
}

type bill struct {
	BillID           int          `json:"billId"`
	ShortTitle       string       `json:"shortTitle"`
	CurrentHouse     string       `json:"currentHouse"`
	OriginatingHouse string       `json:"originatingHouse"`
	IsAct            bool         `json:"isAct"`
	CurrentStage     currentStage `json:"currentStage"`
}

type searchResponse struct {
	Items        []bill `json:"items"`
	TotalResults int    `json:"totalResults"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	query, err := ukgov_common.RequiredString("query", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A search term is required.")
	}
	maxResults := ukgov_common.OptionalInt("max_results", inputs, 20)
	if maxResults <= 0 {
		maxResults = 20
	}
	if maxResults > 20 {
		maxResults = 20
	}

	// Bills API capitalises its paging params.
	q := url.Values{}
	q.Set("SearchTerm", query)
	q.Set("Take", fmt.Sprintf("%d", maxResults))
	endpoint := baseURL + "/api/v1/Bills?" + q.Encode()

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
		"tool_result": summarise(parsed.Items, parsed.TotalResults, query),
		"bills":       parsed.Items,
		"count":       len(parsed.Items),
		"total":       parsed.TotalResults,
		"success":     true,
		"error":       "",
	}, nil
}

func summarise(bills []bill, total int, query string) string {
	if len(bills) == 0 {
		return fmt.Sprintf("No bills found matching %q.", query)
	}
	limit := len(bills)
	if limit > 5 {
		limit = 5
	}
	parts := make([]string, 0, limit)
	for _, b := range bills[:limit] {
		kind := "Bill"
		if b.IsAct {
			kind = "Act"
		}
		stage := b.CurrentStage.Description
		if stage == "" {
			stage = kind
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", b.ShortTitle, stage))
	}
	return fmt.Sprintf("Found %d bill(s) matching %q. Top: %s.", total, query, strings.Join(parts, "; "))
}

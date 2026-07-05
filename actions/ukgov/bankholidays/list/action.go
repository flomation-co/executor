// Package ukgov_bankholidays_list lists UK bank holidays for a region and
// identifies the next upcoming one, from the free GOV.UK bank-holidays feed.
package ukgov_bankholidays_list

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Bank Holidays"
	Description  = "List UK bank holidays for a region, with the next upcoming date (GOV.UK)"
	Website      = "https://www.flomation.co"
	Icon         = "calendar"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

// baseURL is the GOV.UK bank holidays feed root. Package variable so tests can
// point it at a mock server. No authentication required.
var baseURL = "https://www.gov.uk"

var Inputs = [...]core.Connection{
	{
		Name:  "division",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "England & Wales", Value: "england-and-wales"},
			{Name: "Scotland", Value: "scotland"},
			{Name: "Northern Ireland", Value: "northern-ireland"},
		},
		Placeholder: "england-and-wales",
	},
	{Name: "year", Type: core.ConnectionTypeString, Label: "Year (optional filter)", Placeholder: "2026"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "holidays", Type: core.ConnectionTypeObject, Label: "Bank Holidays"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "next_holiday", Type: core.ConnectionTypeString, Label: "Next Holiday"},
	{Name: "next_date", Type: core.ConnectionTypeString, Label: "Next Date"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type event struct {
	Title   string `json:"title"`
	Date    string `json:"date"`
	Notes   string `json:"notes"`
	Bunting bool   `json:"bunting"`
}

type division struct {
	Division string  `json:"division"`
	Events   []event `json:"events"`
}

var validDivisions = map[string]bool{
	"england-and-wales": true,
	"scotland":          true,
	"northern-ireland":  true,
}

const dateLayout = "2006-01-02"

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	div := strings.TrimSpace(ukgov_common.OptionalString("division", inputs))
	if div == "" {
		div = "england-and-wales"
	}
	if !validDivisions[div] {
		return ukgov_common.ErrResult("Unknown region %q — use england-and-wales, scotland or northern-ireland.", div)
	}
	year := strings.TrimSpace(ukgov_common.OptionalString("year", inputs))

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := ukgov_common.Fetch(ctx, http.MethodGet, baseURL+"/bank-holidays.json", nil)
	if err != nil {
		return ukgov_common.ErrResult("GOV.UK bank holidays request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("GOV.UK bank holidays returned status %d", status)
	}

	var parsed map[string]division
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse GOV.UK bank holidays response: %v", err)
	}

	region, ok := parsed[div]
	if !ok {
		return ukgov_common.ErrResult("No bank holiday data for region %q.", div)
	}

	events := region.Events
	if year != "" {
		filtered := make([]event, 0, len(events))
		for _, e := range events {
			if strings.HasPrefix(e.Date, year+"-") {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	nextTitle, nextDate := nextUpcoming(events, time.Now())

	return map[string]interface{}{
		"tool_result":  summarise(events, div, year, nextTitle, nextDate),
		"holidays":     events,
		"count":        len(events),
		"next_holiday": nextTitle,
		"next_date":    nextDate,
		"success":      true,
		"error":        "",
	}, nil
}

// nextUpcoming returns the title and date of the first holiday on or after now.
func nextUpcoming(events []event, now time.Time) (string, string) {
	today := now.Format(dateLayout)
	for _, e := range events {
		if e.Date >= today { // ISO dates sort lexicographically
			return e.Title, e.Date
		}
	}
	return "", ""
}

func summarise(events []event, div, year, nextTitle, nextDate string) string {
	scope := div
	if year != "" {
		scope = fmt.Sprintf("%s in %s", div, year)
	}
	if len(events) == 0 {
		return fmt.Sprintf("No bank holidays found for %s.", scope)
	}
	if nextTitle != "" {
		return fmt.Sprintf("%d bank holiday(s) for %s. Next: %s on %s.", len(events), scope, nextTitle, nextDate)
	}
	return fmt.Sprintf("%d bank holiday(s) for %s.", len(events), scope)
}

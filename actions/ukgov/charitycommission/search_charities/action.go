// Package ukgov_charitycommission_search_charities searches the register of
// charities for England & Wales by name.
package ukgov_charitycommission_search_charities

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/charitycommission"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Search Charities"
	Description  = "Search the England & Wales register of charities by name (Charity Commission)"
	Website      = "https://www.flomation.co"
	Icon         = "hand+magnifying-glass"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Charity Commission Subscription Key", Placeholder: "${secrets.CHARITY_COMMISSION_KEY}", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Charity Name", Placeholder: "e.g. Oxfam", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "charities", Type: core.ConnectionTypeObject, Label: "Charities"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type charity struct {
	OrganisationNumber int    `json:"organisation_number"`
	RegCharityNumber   int    `json:"reg_charity_number"`
	CharityName        string `json:"charity_name"`
	RegStatus          string `json:"reg_status"`
	DateOfRegistration string `json:"date_of_registration"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := ukgov_common.RequiredString("api_key", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A Charity Commission subscription key is required.")
	}
	name, err := ukgov_common.RequiredString("name", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A charity name to search for is required.")
	}

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := charitycommission.Get(ctx, apiKey, "/searchCharityName/"+url.PathEscape(name))
	if err != nil {
		return ukgov_common.ErrResult("Charity Commission request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("%s", charitycommission.StatusMessage(status))
	}

	// Search returns a bare JSON array.
	var charities []charity
	if err := json.Unmarshal(body, &charities); err != nil {
		return ukgov_common.ErrResult("Failed to parse Charity Commission response: %v", err)
	}

	return map[string]interface{}{
		"tool_result": summarise(charities, name),
		"charities":   charities,
		"count":       len(charities),
		"success":     true,
		"error":       "",
	}, nil
}

func summarise(charities []charity, name string) string {
	if len(charities) == 0 {
		return fmt.Sprintf("No charities found matching %q.", name)
	}
	limit := len(charities)
	if limit > 5 {
		limit = 5
	}
	parts := make([]string, 0, limit)
	for _, c := range charities[:limit] {
		parts = append(parts, fmt.Sprintf("%s (%d, %s)", c.CharityName, c.RegCharityNumber, c.RegStatus))
	}
	return fmt.Sprintf("Found %d charities matching %q. Top: %s.", len(charities), name, strings.Join(parts, "; "))
}

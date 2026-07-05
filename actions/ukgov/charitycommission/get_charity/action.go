// Package ukgov_charitycommission_get_charity retrieves the full register
// details for a charity by its registered number.
//
// The allcharitydetailsV2 response schema is not published as a static spec, so
// this action returns the raw details object for downstream use and extracts a
// name/status summary defensively across likely key spellings. Verify exact
// keys against the developer portal once a live subscription key is available.
package ukgov_charitycommission_get_charity

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
	Name         = "Get Charity"
	Description  = "Retrieve full register details for a charity by registered number (Charity Commission)"
	Website      = "https://www.flomation.co"
	Icon         = "hand"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Charity Commission Subscription Key", Placeholder: "${secrets.CHARITY_COMMISSION_KEY}", Required: true},
	{Name: "charity_number", Type: core.ConnectionTypeString, Label: "Registered Charity Number", Placeholder: "e.g. 202918", Required: true},
	{Name: "suffix", Type: core.ConnectionTypeString, Label: "Group/Subsidiary Suffix", Placeholder: "0"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "charity", Type: core.ConnectionTypeObject, Label: "Charity Details"},
	{Name: "charity_name", Type: core.ConnectionTypeString, Label: "Charity Name"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Registration Status"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := ukgov_common.RequiredString("api_key", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A Charity Commission subscription key is required.")
	}
	number, err := ukgov_common.RequiredString("charity_number", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A registered charity number is required.")
	}
	suffix := strings.TrimSpace(ukgov_common.OptionalString("suffix", inputs))
	if suffix == "" {
		suffix = "0"
	}

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	path := "/allcharitydetailsV2/" + url.PathEscape(number) + "/" + url.PathEscape(suffix)
	status, body, err := charitycommission.Get(ctx, apiKey, path)
	if err != nil {
		return ukgov_common.ErrResult("Charity Commission request failed: %v", err)
	}
	if status == http.StatusNotFound {
		return ukgov_common.ErrResult("No charity found with registered number %s.", number)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("%s", charitycommission.StatusMessage(status))
	}

	var details map[string]interface{}
	if err := json.Unmarshal(body, &details); err != nil {
		return ukgov_common.ErrResult("Failed to parse Charity Commission response: %v", err)
	}

	name := stringField(details, "charity_name", "organisation_name")
	regStatus := stringField(details, "reg_status", "registration_status", "charity_registration_status")

	summary := fmt.Sprintf("Charity %s", number)
	if name != "" {
		summary = fmt.Sprintf("%s (%s)", name, number)
	}
	if regStatus != "" {
		summary += " — " + regStatus
	}
	summary += "."

	return map[string]interface{}{
		"tool_result":  summary,
		"charity":      details,
		"charity_name": name,
		"status":       regStatus,
		"success":      true,
		"error":        "",
	}, nil
}

// stringField returns the first non-empty string value among the given keys.
func stringField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

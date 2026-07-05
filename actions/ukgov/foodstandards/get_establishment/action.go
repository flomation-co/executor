// Package ukgov_foodstandards_get_establishment looks up a single food
// establishment (and its hygiene rating) by FHRS ID. No authentication needed.
package ukgov_foodstandards_get_establishment

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
	Name         = "Get Food Establishment"
	Description  = "Look up a UK food hygiene rating by FHRS establishment ID (Food Standards Agency)"
	Website      = "https://www.flomation.co"
	Icon         = "star"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "fhrs_id",
		Type:        core.ConnectionTypeString,
		Label:       "FHRS ID",
		Placeholder: "e.g. 512112",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "establishment", Type: core.ConnectionTypeObject, Label: "Establishment"},
	{Name: "business_name", Type: core.ConnectionTypeString, Label: "Business Name"},
	{Name: "rating_value", Type: core.ConnectionTypeString, Label: "Hygiene Rating"},
	{Name: "address", Type: core.ConnectionTypeString, Label: "Address"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	id, err := ukgov_common.RequiredString("fhrs_id", inputs)
	if err != nil {
		return ukgov_common.ErrResult("An FHRS ID is required.")
	}

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := foodstandards.Get(ctx, "/Establishments/"+url.PathEscape(id), nil)
	if err != nil {
		return ukgov_common.ErrResult("Food Standards Agency request failed: %v", err)
	}
	if status == http.StatusNotFound {
		return ukgov_common.ErrResult("No food establishment found with FHRS ID %s.", id)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Food Standards Agency returned status %d", status)
	}

	var e foodstandards.Establishment
	if err := json.Unmarshal(body, &e); err != nil {
		return ukgov_common.ErrResult("Failed to parse Food Standards Agency response: %v", err)
	}

	rating := strings.TrimSpace(e.RatingValue)
	if rating == "" {
		rating = "unrated"
	}
	summary := fmt.Sprintf("%s — food hygiene rating %s. %s.", e.BusinessName, rating, e.Address())

	return map[string]interface{}{
		"tool_result":   summary,
		"establishment": e,
		"business_name": e.BusinessName,
		"rating_value":  e.RatingValue,
		"address":       e.Address(),
		"success":       true,
		"error":         "",
	}, nil
}

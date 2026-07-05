// Package ukgov_foodstandards_list_business_types lists the food business type
// categories used by the FHRS scheme (useful for filtering searches). No auth.
package ukgov_foodstandards_list_business_types

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/foodstandards"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Food Business Types"
	Description  = "List the food business type categories used by the FHRS scheme (Food Standards Agency)"
	Website      = "https://www.flomation.co"
	Icon         = "star+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "business_types", Type: core.ConnectionTypeObject, Label: "Business Types"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type businessType struct {
	ID   int64  `json:"BusinessTypeId"`
	Name string `json:"BusinessTypeName"`
}

type businessTypesResponse struct {
	BusinessTypes []businessType `json:"businessTypes"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := foodstandards.Get(ctx, "/BusinessTypes", nil)
	if err != nil {
		return ukgov_common.ErrResult("Food Standards Agency request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Food Standards Agency returned status %d", status)
	}

	var parsed businessTypesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse Food Standards Agency response: %v", err)
	}

	names := make([]string, 0, len(parsed.BusinessTypes))
	for _, bt := range parsed.BusinessTypes {
		names = append(names, bt.Name)
	}

	summary := fmt.Sprintf("%d food business types available: %s.", len(names), strings.Join(names, ", "))
	if len(names) == 0 {
		summary = "No food business types returned."
	}

	return map[string]interface{}{
		"tool_result":    summary,
		"business_types": parsed.BusinessTypes,
		"count":          len(parsed.BusinessTypes),
		"success":        true,
		"error":          "",
	}, nil
}

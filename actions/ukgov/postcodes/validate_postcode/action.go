// Package ukgov_postcodes_validate_postcode checks whether a string is a valid
// UK postcode via the free postcodes.io API.
package ukgov_postcodes_validate_postcode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/postcodes"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Validate Postcode"
	Description  = "Check whether a string is a valid UK postcode (postcodes.io)"
	Website      = "https://www.flomation.co"
	Icon         = "location-dot+circle-check"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "postcode", Type: core.ConnectionTypeString, Label: "Postcode", Placeholder: "SW1A 1AA", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "valid", Type: core.ConnectionTypeBoolean, Label: "Is Valid"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type validateResponse struct {
	Status int  `json:"status"`
	Result bool `json:"result"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	pc, err := ukgov_common.RequiredString("postcode", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A postcode is required.")
	}

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := postcodes.Get(ctx, "/postcodes/"+url.PathEscape(pc)+"/validate", nil)
	if err != nil {
		return ukgov_common.ErrResult("Postcode validation request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Postcode validation returned status %d", status)
	}

	var parsed validateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse postcode validation response: %v", err)
	}

	verb := "is a valid"
	if !parsed.Result {
		verb = "is not a valid"
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%q %s UK postcode.", pc, verb),
		"valid":       parsed.Result,
		"success":     true,
		"error":       "",
	}, nil
}

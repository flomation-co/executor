// Package ukgov_postcodes_nearest_postcodes lists postcodes geographically
// nearest to a given UK postcode via the free postcodes.io API.
package ukgov_postcodes_nearest_postcodes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/postcodes"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Nearest Postcodes"
	Description  = "List UK postcodes geographically nearest to a given postcode (postcodes.io)"
	Website      = "https://www.flomation.co"
	Icon         = "map+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "postcode", Type: core.ConnectionTypeString, Label: "Postcode", Placeholder: "SW1A 1AA", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Nearest Postcodes"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type nearestResponse struct {
	Status int                  `json:"status"`
	Result []postcodes.Postcode `json:"result"`
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

	status, body, err := postcodes.Get(ctx, "/postcodes/"+url.PathEscape(pc)+"/nearest", nil)
	if err != nil {
		return ukgov_common.ErrResult("Nearest postcodes request failed: %v", err)
	}
	if status == http.StatusNotFound {
		return ukgov_common.ErrResult("%q is not a recognised UK postcode.", pc)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Nearest postcodes returned status %d", status)
	}

	var parsed nearestResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse nearest postcodes response: %v", err)
	}

	names := make([]string, 0, len(parsed.Result))
	for _, p := range parsed.Result {
		names = append(names, p.Postcode)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%d postcode(s) near %s: %s.", len(names), pc, strings.Join(names, ", ")),
		"results":     parsed.Result,
		"count":       len(parsed.Result),
		"success":     true,
		"error":       "",
	}, nil
}

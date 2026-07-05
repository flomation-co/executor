// Package ukgov_police_crime_categories lists the crime categories valid for a
// given month in the data.police.uk API. No authentication required.
package ukgov_police_crime_categories

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/police"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Crime Categories"
	Description  = "List UK crime categories valid for a given month (Police UK)"
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+tags"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "date", Type: core.ConnectionTypeString, Label: "Month (YYYY-MM, optional)", Placeholder: "2024-01"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "categories", Type: core.ConnectionTypeObject, Label: "Categories"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type category struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	date := ukgov_common.OptionalString("date", inputs)

	q := url.Values{}
	if date != "" {
		q.Set("date", date)
	}

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := police.Get(ctx, "/crime-categories", q)
	if err != nil {
		return ukgov_common.ErrResult("Police UK request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Police UK returned status %d", status)
	}

	var categories []category
	if err := json.Unmarshal(body, &categories); err != nil {
		return ukgov_common.ErrResult("Failed to parse Police UK response: %v", err)
	}

	names := make([]string, 0, len(categories))
	for _, c := range categories {
		names = append(names, c.Name)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%d crime categories available: %s.", len(categories), strings.Join(names, ", ")),
		"categories":  categories,
		"count":       len(categories),
		"success":     true,
		"error":       "",
	}, nil
}

// Package ukgov_companieshouse_list_psc lists a company's persons with
// significant control (beneficial owners) from the UK Companies House register.
package ukgov_companieshouse_list_psc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/companieshouse"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Persons with Significant Control"
	Description  = "List a UK company's beneficial owners / persons with significant control (Companies House)"
	Website      = "https://www.flomation.co"
	Icon         = "user-group"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Companies House API Key", Placeholder: "${secrets.COMPANIES_HOUSE_KEY}", Required: true},
	{Name: "company_number", Type: core.ConnectionTypeString, Label: "Company Number", Placeholder: "e.g. 12345678", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "persons", Type: core.ConnectionTypeObject, Label: "Persons with Significant Control"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type psc struct {
	Name             string   `json:"name"`
	Kind             string   `json:"kind"`
	NaturesOfControl []string `json:"natures_of_control"`
	NotifiedOn       string   `json:"notified_on"`
	CeasedOn         string   `json:"ceased_on"`
	Nationality      string   `json:"nationality"`
}

type pscResponse struct {
	Items        []psc `json:"items"`
	ActiveCount  int   `json:"active_count"`
	TotalResults int   `json:"total_results"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := ukgov_common.RequiredString("api_key", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A Companies House API key is required.")
	}
	number, err := ukgov_common.RequiredString("company_number", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A company number is required.")
	}

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := companieshouse.Get(ctx, apiKey, "/company/"+url.PathEscape(number)+"/persons-with-significant-control", nil)
	if err != nil {
		return ukgov_common.ErrResult("Companies House request failed: %v", err)
	}
	// A 404 here means the company has no PSC register — a valid result, not a
	// failure, so report zero rather than erroring.
	if status == http.StatusNotFound {
		return noneResult(number)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("%s", companieshouse.StatusMessage(status))
	}

	var parsed pscResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse Companies House response: %v", err)
	}
	if len(parsed.Items) == 0 {
		return noneResult(number)
	}

	limit := len(parsed.Items)
	if limit > 5 {
		limit = 5
	}
	parts := make([]string, 0, limit)
	for _, p := range parsed.Items[:limit] {
		parts = append(parts, fmt.Sprintf("%s [%s]", p.Name, strings.Join(p.NaturesOfControl, ", ")))
	}
	summary := fmt.Sprintf("Company %s has %d person(s) with significant control. E.g. %s.",
		number, parsed.TotalResults, strings.Join(parts, "; "))

	return map[string]interface{}{
		"tool_result": summary,
		"persons":     parsed.Items,
		"count":       len(parsed.Items),
		"success":     true,
		"error":       "",
	}, nil
}

func noneResult(number string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Company %s has no persons with significant control on record.", number),
		"persons":     []psc{},
		"count":       0,
		"success":     true,
		"error":       "",
	}, nil
}

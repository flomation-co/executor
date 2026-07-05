// Package ukgov_companieshouse_get_insolvency retrieves a company's insolvency
// case history from the UK Companies House register.
package ukgov_companieshouse_get_insolvency

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
	Name         = "Get Insolvency"
	Description  = "Retrieve a UK company's insolvency case history (Companies House)"
	Website      = "https://www.flomation.co"
	Icon         = "triangle-exclamation"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Companies House API Key", Placeholder: "${secrets.COMPANIES_HOUSE_KEY}", Required: true},
	{Name: "company_number", Type: core.ConnectionTypeString, Label: "Company Number", Placeholder: "e.g. 12345678", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "cases", Type: core.ConnectionTypeObject, Label: "Insolvency Cases"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Case Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type caseDate struct {
	Date string `json:"date"`
	Type string `json:"type"`
}

type practitioner struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type insolvencyCase struct {
	Number        string         `json:"number"`
	Type          string         `json:"type"`
	Dates         []caseDate     `json:"dates"`
	Practitioners []practitioner `json:"practitioners"`
}

type insolvencyResponse struct {
	Cases []insolvencyCase `json:"cases"`
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

	status, body, err := companieshouse.Get(ctx, apiKey, "/company/"+url.PathEscape(number)+"/insolvency", nil)
	if err != nil {
		return ukgov_common.ErrResult("Companies House request failed: %v", err)
	}
	// A 404 here means the company has no insolvency history — a valid result.
	if status == http.StatusNotFound {
		return noneResult(number)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("%s", companieshouse.StatusMessage(status))
	}

	var parsed insolvencyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse Companies House response: %v", err)
	}
	if len(parsed.Cases) == 0 {
		return noneResult(number)
	}

	types := make([]string, 0, len(parsed.Cases))
	for _, c := range parsed.Cases {
		label := c.Type
		if c.Number != "" {
			label += " (" + c.Number + ")"
		}
		types = append(types, label)
	}
	summary := fmt.Sprintf("Company %s has %d insolvency case(s): %s.", number, len(parsed.Cases), strings.Join(types, "; "))

	return map[string]interface{}{
		"tool_result": summary,
		"cases":       parsed.Cases,
		"count":       len(parsed.Cases),
		"success":     true,
		"error":       "",
	}, nil
}

func noneResult(number string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Company %s has no insolvency history.", number),
		"cases":       []insolvencyCase{},
		"count":       0,
		"success":     true,
		"error":       "",
	}, nil
}

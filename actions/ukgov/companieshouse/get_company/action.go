// Package ukgov_companieshouse_get_company retrieves a company's profile from
// the UK Companies House register by company number.
package ukgov_companieshouse_get_company

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
	Name         = "Get Company"
	Description  = "Retrieve a UK company's profile by company number (Companies House)"
	Website      = "https://www.flomation.co"
	Icon         = "building+circle-info"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Companies House API Key", Placeholder: "${secrets.COMPANIES_HOUSE_KEY}", Required: true},
	{Name: "company_number", Type: core.ConnectionTypeString, Label: "Company Number", Placeholder: "e.g. 12345678", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "company", Type: core.ConnectionTypeObject, Label: "Company Profile"},
	{Name: "company_name", Type: core.ConnectionTypeString, Label: "Company Name"},
	{Name: "company_status", Type: core.ConnectionTypeString, Label: "Company Status"},
	{Name: "registered_office", Type: core.ConnectionTypeString, Label: "Registered Office"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// profile mirrors the Companies House profile resource. Note the company type
// is keyed `type` here (search results use `company_type`).
type profile struct {
	CompanyName             string                 `json:"company_name"`
	CompanyNumber           string                 `json:"company_number"`
	CompanyStatus           string                 `json:"company_status"`
	Type                    string                 `json:"type"`
	DateOfCreation          string                 `json:"date_of_creation"`
	SICCodes                []string               `json:"sic_codes"`
	HasCharges              bool                   `json:"has_charges"`
	HasInsolvencyHistory    bool                   `json:"has_insolvency_history"`
	RegisteredOfficeAddress companieshouse.Address `json:"registered_office_address"`
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

	status, body, err := companieshouse.Get(ctx, apiKey, "/company/"+url.PathEscape(number), nil)
	if err != nil {
		return ukgov_common.ErrResult("Companies House request failed: %v", err)
	}
	if status == http.StatusNotFound {
		return ukgov_common.ErrResult("No company found with number %s.", number)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("%s", companieshouse.StatusMessage(status))
	}

	var p profile
	if err := json.Unmarshal(body, &p); err != nil {
		return ukgov_common.ErrResult("Failed to parse Companies House response: %v", err)
	}

	office := p.RegisteredOfficeAddress.OneLine()
	summary := fmt.Sprintf("%s (%s) — %s.", p.CompanyName, p.CompanyNumber, p.CompanyStatus)
	if p.Type != "" {
		summary += " Type: " + p.Type + "."
	}
	if p.DateOfCreation != "" {
		summary += " Incorporated " + p.DateOfCreation + "."
	}
	if office != "" {
		summary += " Registered office: " + office + "."
	}
	if extras := flags(p); extras != "" {
		summary += " " + extras
	}

	return map[string]interface{}{
		"tool_result":       summary,
		"company":           p,
		"company_name":      p.CompanyName,
		"company_status":    p.CompanyStatus,
		"registered_office": office,
		"success":           true,
		"error":             "",
	}, nil
}

// flags surfaces the due-diligence booleans an AI agent cares about.
func flags(p profile) string {
	var notes []string
	if p.HasCharges {
		notes = append(notes, "has registered charges")
	}
	if p.HasInsolvencyHistory {
		notes = append(notes, "has insolvency history")
	}
	if len(notes) == 0 {
		return ""
	}
	return "Note: " + strings.Join(notes, "; ") + "."
}

// Package ukgov_companieshouse_list_officers lists a company's officers
// (directors, secretaries) from the UK Companies House register.
package ukgov_companieshouse_list_officers

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
	Name         = "List Officers"
	Description  = "List a UK company's officers — directors and secretaries (Companies House)"
	Website      = "https://www.flomation.co"
	Icon         = "people-group"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Companies House API Key", Placeholder: "${secrets.COMPANIES_HOUSE_KEY}", Required: true},
	{Name: "company_number", Type: core.ConnectionTypeString, Label: "Company Number", Placeholder: "e.g. 12345678", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "officers", Type: core.ConnectionTypeObject, Label: "Officers"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned Count"},
	{Name: "active_count", Type: core.ConnectionTypeInteger, Label: "Active Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type officer struct {
	Name        string `json:"name"`
	OfficerRole string `json:"officer_role"`
	AppointedOn string `json:"appointed_on"`
	ResignedOn  string `json:"resigned_on"`
	Nationality string `json:"nationality"`
	Occupation  string `json:"occupation"`
}

type officersResponse struct {
	Items         []officer `json:"items"`
	ActiveCount   int       `json:"active_count"`
	ResignedCount int       `json:"resigned_count"`
	TotalResults  int       `json:"total_results"`
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

	status, body, err := companieshouse.Get(ctx, apiKey, "/company/"+url.PathEscape(number)+"/officers", nil)
	if err != nil {
		return ukgov_common.ErrResult("Companies House request failed: %v", err)
	}
	if status == http.StatusNotFound {
		return ukgov_common.ErrResult("No company found with number %s.", number)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("%s", companieshouse.StatusMessage(status))
	}

	var parsed officersResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse Companies House response: %v", err)
	}

	return map[string]interface{}{
		"tool_result":  summarise(parsed, number),
		"officers":     parsed.Items,
		"count":        len(parsed.Items),
		"active_count": parsed.ActiveCount,
		"success":      true,
		"error":        "",
	}, nil
}

func summarise(r officersResponse, number string) string {
	if len(r.Items) == 0 {
		return fmt.Sprintf("Company %s has no listed officers.", number)
	}
	limit := len(r.Items)
	if limit > 5 {
		limit = 5
	}
	parts := make([]string, 0, limit)
	for _, o := range r.Items[:limit] {
		state := "active"
		if o.ResignedOn != "" {
			state = "resigned"
		}
		parts = append(parts, fmt.Sprintf("%s (%s, %s)", o.Name, o.OfficerRole, state))
	}
	return fmt.Sprintf("Company %s has %d officer(s) (%d active, %d resigned). E.g. %s.",
		number, r.TotalResults, r.ActiveCount, r.ResignedCount, strings.Join(parts, "; "))
}

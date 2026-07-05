// Package ukgov_companieshouse_list_charges lists a company's registered
// charges (mortgages, debentures) from the UK Companies House register.
package ukgov_companieshouse_list_charges

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
	Name         = "List Charges"
	Description  = "List a UK company's registered charges — mortgages and debentures (Companies House)"
	Website      = "https://www.flomation.co"
	Icon         = "building+file-contract"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Companies House API Key", Placeholder: "${secrets.COMPANIES_HOUSE_KEY}", Required: true},
	{Name: "company_number", Type: core.ConnectionTypeString, Label: "Company Number", Placeholder: "e.g. 12345678", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "charges", Type: core.ConnectionTypeObject, Label: "Charges"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type classification struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type charge struct {
	ChargeCode     string         `json:"charge_code"`
	Status         string         `json:"status"`
	CreatedOn      string         `json:"created_on"`
	DeliveredOn    string         `json:"delivered_on"`
	Classification classification `json:"classification"`
}

type chargesResponse struct {
	Items          []charge `json:"items"`
	TotalCount     int      `json:"total_count"`
	SatisfiedCount int      `json:"satisfied_count"`
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

	status, body, err := companieshouse.Get(ctx, apiKey, "/company/"+url.PathEscape(number)+"/charges", nil)
	if err != nil {
		return ukgov_common.ErrResult("Companies House request failed: %v", err)
	}
	if status == http.StatusNotFound {
		return noneResult(number)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("%s", companieshouse.StatusMessage(status))
	}

	var parsed chargesResponse
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
	for _, c := range parsed.Items[:limit] {
		parts = append(parts, fmt.Sprintf("%s — %s (%s)", c.Classification.Description, c.Status, c.CreatedOn))
	}
	summary := fmt.Sprintf("Company %s has %d charge(s), %d satisfied. E.g. %s.",
		number, parsed.TotalCount, parsed.SatisfiedCount, strings.Join(parts, "; "))

	return map[string]interface{}{
		"tool_result": summary,
		"charges":     parsed.Items,
		"count":       len(parsed.Items),
		"total":       parsed.TotalCount,
		"success":     true,
		"error":       "",
	}, nil
}

func noneResult(number string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Company %s has no registered charges.", number),
		"charges":     []charge{},
		"count":       0,
		"total":       0,
		"success":     true,
		"error":       "",
	}, nil
}

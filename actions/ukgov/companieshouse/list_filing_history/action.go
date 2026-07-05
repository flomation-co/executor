// Package ukgov_companieshouse_list_filing_history lists a company's filing
// history from the UK Companies House register.
package ukgov_companieshouse_list_filing_history

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/companieshouse"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Filing History"
	Description  = "List a UK company's filing history — accounts, confirmations, changes (Companies House)"
	Website      = "https://www.flomation.co"
	Icon         = "building+file-lines"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Companies House API Key", Placeholder: "${secrets.COMPANIES_HOUSE_KEY}", Required: true},
	{Name: "company_number", Type: core.ConnectionTypeString, Label: "Company Number", Placeholder: "e.g. 12345678", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "filings", Type: core.ConnectionTypeObject, Label: "Filings"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type filing struct {
	TransactionID string `json:"transaction_id"`
	Category      string `json:"category"`
	Type          string `json:"type"`
	Date          string `json:"date"`
	Description   string `json:"description"`
}

// filingResponse uses total_count (not total_results) per the Companies House
// filing-history resource.
type filingResponse struct {
	Items      []filing `json:"items"`
	TotalCount int      `json:"total_count"`
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

	status, body, err := companieshouse.Get(ctx, apiKey, "/company/"+url.PathEscape(number)+"/filing-history", nil)
	if err != nil {
		return ukgov_common.ErrResult("Companies House request failed: %v", err)
	}
	if status == http.StatusNotFound {
		return ukgov_common.ErrResult("No company found with number %s.", number)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("%s", companieshouse.StatusMessage(status))
	}

	var parsed filingResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse Companies House response: %v", err)
	}

	summary := fmt.Sprintf("Company %s has no filing history.", number)
	if len(parsed.Items) > 0 {
		latest := parsed.Items[0]
		summary = fmt.Sprintf("Company %s has %d filing(s). Latest: %s — %s.",
			number, parsed.TotalCount, latest.Date, latest.Description)
	}

	return map[string]interface{}{
		"tool_result": summary,
		"filings":     parsed.Items,
		"count":       len(parsed.Items),
		"total":       parsed.TotalCount,
		"success":     true,
		"error":       "",
	}, nil
}

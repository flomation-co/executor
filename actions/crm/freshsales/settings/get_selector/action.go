// Package get_selector implements the Freshsales "Settings: Get Selector" action.
//
// Freshsales exposes fourteen near-identical configuration endpoints — owners,
// deal stages, currencies and so on — each returning a list of ids and names.
// Fourteen separate actions would be fourteen near-identical entries in the Add
// Node menu, so this is one action with the endpoint as a dropdown. The AI sees
// the same choice as an enum on a single tool.
//
// These ids are what every other action's *_id input needs, so this is usually
// the first Freshsales node in a new flow.
package get_selector

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	freshsales_common "flomation.app/automate/executor/actions/crm/freshsales"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Settings: Get Selector"
	Description  = "Read a Freshsales configuration list — owners, deal stages, currencies, sources and more."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+bolt"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "selector", Type: core.ConnectionTypeString, Label: "Which configuration list to read", Required: true, Options: []core.ConnectionOption{
		{Name: "Owners (users)", Value: "owners"},
		{Name: "Territories", Value: "territories"},
		{Name: "Deal stages", Value: "deal_stages"},
		{Name: "Deal types", Value: "deal_types"},
		{Name: "Deal reasons", Value: "deal_reasons"},
		{Name: "Deal payment statuses", Value: "deal_payment_statuses"},
		{Name: "Currencies", Value: "currencies"},
		{Name: "Lead sources", Value: "lead_sources"},
		{Name: "Industry types", Value: "industry_types"},
		{Name: "Business types", Value: "business_types"},
		{Name: "Campaigns", Value: "campaigns"},
		{Name: "Contact statuses", Value: "contact_statuses"},
		{Name: "Lifecycle stages", Value: "lifecycle_stages"},
		{Name: "Sales activity types", Value: "sales_activity_types"},
		{Name: "Sales activity outcomes", Value: "sales_activity_outcomes"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Options"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	client, err := freshsales_common.Client(inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	selector, err := freshsales_common.RequiredString("selector", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	if !freshsales_common.IsKnownSelector(selector) {
		return freshsales_common.ErrorResult(fmt.Sprintf(
			"%q is not a Freshsales selector — pick one from the dropdown", selector)), nil
	}

	resp, err := client.Do(flow, http.MethodGet, "/selector/"+selector, nil, url.Values(nil))
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	// Each endpoint keys its array by its own name, so read the selector's key
	// rather than guessing, and fall back to the whole response.
	items := freshsales_common.Arr(resp, selector)
	out := freshsales_common.ListResult(items, fmt.Sprintf("%s: %d option(s)", selector, len(items)))
	out["result"] = resp
	return out, nil
}

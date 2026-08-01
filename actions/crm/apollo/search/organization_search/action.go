package organization_search

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Companies: Search"
	Description  = "Search Apollo's company database by name, location and headcount."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+magnifying-glass"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "q_organization_name", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Search by company name"},
	{Name: "organization_locations", Type: core.ConnectionTypeString, Label: "Company Locations", Placeholder: "United Kingdom (comma-separated)"},
	{Name: "organization_num_employees_ranges", Type: core.ConnectionTypeString, Label: "Headcount Ranges", Placeholder: "1,10 · 11,50 · 51,200 (comma-separated ranges)"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1"},
	{Name: "per_page", Type: core.ConnectionTypeInteger, Label: "Per Page", Placeholder: "25 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Companies"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	apollo_common.SetString(body, "q_organization_name", "q_organization_name", inputs)
	apollo_common.SetList(body, "organization_locations", "organization_locations", inputs)
	apollo_common.SetList(body, "organization_num_employees_ranges", "organization_num_employees_ranges", inputs)
	apollo_common.SetInt(body, "page", "page", inputs)
	apollo_common.SetInt(body, "per_page", "per_page", inputs)

	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/mixed_companies/search", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	// mixed_companies/search returns matches under "organizations" (or "accounts"
	// when the workspace has them saved); prefer organizations, fall back.
	orgs := apollo_common.Arr(resp, "organizations")
	if len(orgs) == 0 {
		orgs = apollo_common.Arr(resp, "accounts")
	}
	return apollo_common.ListResult(orgs, fmt.Sprintf("Found %d companies", len(orgs))), nil
}

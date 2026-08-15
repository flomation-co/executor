package organization_search

import (
	"fmt"
	"net/url"

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
	{Name: "organization_locations", Type: core.ConnectionTypeString, Label: "Company Locations", Placeholder: "Chester, United Kingdom — use a CITY (UK counties are often unknown to Apollo and get ignored); separate multiple with ;"},
	{Name: "organization_num_employees_ranges", Type: core.ConnectionTypeString, Label: "Headcount Ranges", Placeholder: "Each range as min,max — separate ranges with ; e.g. 1,10;11,50;51,5000"},
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

	// Apollo reads company-search filters from the URL query string, not the
	// JSON body — sending them in the body made Apollo ignore the location and
	// name filters and return a generic list. See the query-parameter builders.
	q := url.Values{}
	apollo_common.AddQueryString(q, "q_organization_name", "q_organization_name", inputs)
	// Locations are semicolon-separated: a comma is part of a single Apollo
	// location value ("Chester, United Kingdom"), and splitting on it ORs a city
	// with its country, silently widening the search to the whole country.
	apollo_common.AddQueryLocationList(q, "organization_locations", "organization_locations", inputs)
	apollo_common.AddQueryRangeList(q, "organization_num_employees_ranges", "organization_num_employees_ranges", inputs)
	apollo_common.AddQueryInt(q, "page", "page", inputs)
	apollo_common.AddQueryInt(q, "per_page", "per_page", inputs)

	resp, err := apollo_common.NewClient(apiKey).PostQuery(flow, "/mixed_companies/search", q)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	// mixed_companies/search returns matches under "organizations" (or "accounts"
	// when the workspace has them saved); prefer organizations, fall back.
	orgs := apollo_common.Arr(resp, "organizations")
	if len(orgs) == 0 {
		orgs = apollo_common.Arr(resp, "accounts")
	}
	// Apollo drops location values outside its taxonomy and answers an
	// unfiltered search instead, which is indistinguishable from a real result
	// until you read the companies. Say so when nothing matches.
	summary := fmt.Sprintf("Found %d companies", len(orgs))
	if warn := apollo_common.LocationIgnoredWarning(
		apollo_common.LocationList("organization_locations", inputs),
		apollo_common.OrgLocations(orgs),
	); warn != "" {
		summary = warn + "\n" + summary
	}
	return apollo_common.ListResult(orgs, summary), nil
}

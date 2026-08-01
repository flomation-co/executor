package people_search

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "People: Search"
	Description  = "Search Apollo's people database by title, seniority, location and keywords."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+magnifying-glass"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "q_keywords", Type: core.ConnectionTypeString, Label: "Keywords", Placeholder: "Free-text search across name, title, company"},
	{Name: "person_titles", Type: core.ConnectionTypeString, Label: "Job Titles", Placeholder: "CEO, Head of Sales (comma-separated)"},
	{Name: "person_seniorities", Type: core.ConnectionTypeString, Label: "Seniorities", Placeholder: "owner, founder, c_suite, vp, director (comma-separated)"},
	{Name: "person_locations", Type: core.ConnectionTypeString, Label: "Person Locations", Placeholder: "London, United Kingdom (comma-separated)"},
	{Name: "organization_domains", Type: core.ConnectionTypeString, Label: "Company Domains", Placeholder: "example.com (comma-separated)"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1"},
	{Name: "per_page", Type: core.ConnectionTypeInteger, Label: "Per Page", Placeholder: "25 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "People"},
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
	apollo_common.SetString(body, "q_keywords", "q_keywords", inputs)
	apollo_common.SetList(body, "person_titles", "person_titles", inputs)
	apollo_common.SetList(body, "person_seniorities", "person_seniorities", inputs)
	apollo_common.SetList(body, "person_locations", "person_locations", inputs)
	apollo_common.SetList(body, "organization_domains", "organization_domains", inputs)
	apollo_common.SetInt(body, "page", "page", inputs)
	apollo_common.SetInt(body, "per_page", "per_page", inputs)

	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/mixed_people/search", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	people := apollo_common.Arr(resp, "people")
	return apollo_common.ListResult(people, fmt.Sprintf("Found %d people", len(people))), nil
}

package contact_search

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Contacts: Search"
	Description  = "Search the contacts saved in your Apollo CRM by keyword, stage and list."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+magnifying-glass"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "q_keywords", Type: core.ConnectionTypeString, Label: "Keywords", Placeholder: "Name, email or company"},
	{Name: "contact_stage_ids", Type: core.ConnectionTypeString, Label: "Stage IDs", Placeholder: "Comma-separated contact stage IDs"},
	{Name: "contact_label_ids", Type: core.ConnectionTypeString, Label: "List IDs", Placeholder: "Comma-separated list (label) IDs"},
	{Name: "sort_by_field", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "e.g. contact_last_activity_date"},
	{Name: "sort_ascending", Type: core.ConnectionTypeBoolean, Label: "Sort Ascending"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1"},
	{Name: "per_page", Type: core.ConnectionTypeInteger, Label: "Per Page", Placeholder: "25 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Contacts"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	// Apollo reads contact-search filters from the URL query string, not the body.
	q := url.Values{}
	apollo_common.AddQueryString(q, "q_keywords", "q_keywords", inputs)
	apollo_common.AddQueryList(q, "contact_stage_ids", "contact_stage_ids", inputs)
	apollo_common.AddQueryList(q, "contact_label_ids", "contact_label_ids", inputs)
	apollo_common.AddQueryString(q, "sort_by_field", "sort_by_field", inputs)
	apollo_common.AddQueryBool(q, "sort_ascending", "sort_ascending", inputs)
	apollo_common.AddQueryInt(q, "page", "page", inputs)
	apollo_common.AddQueryInt(q, "per_page", "per_page", inputs)

	resp, err := apollo_common.NewClient(apiKey).PostQuery(flow, "/contacts/search", q)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	contacts := apollo_common.Arr(resp, "contacts")
	return apollo_common.ListResult(contacts, fmt.Sprintf("Found %d contacts", len(contacts))), nil
}

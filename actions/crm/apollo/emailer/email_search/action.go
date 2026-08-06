package email_search

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Emails: Search"
	Description  = "Search Apollo outreach emails by contact, sequence or keyword. Master key required."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+magnifying-glass"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey} (master key)", Required: true},
	{Name: "q_keywords", Type: core.ConnectionTypeString, Label: "Keywords", Placeholder: "Subject / body search"},
	{Name: "emailer_campaign_id", Type: core.ConnectionTypeString, Label: "Sequence ID", Placeholder: "Filter to one sequence"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "Filter to one contact"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1"},
	{Name: "per_page", Type: core.ConnectionTypeInteger, Label: "Per Page", Placeholder: "25 (max 100)"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Filters (JSON)", Placeholder: `{"email_status":["sent"]}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Emails"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	// Apollo reads emailer-message search filters from the URL query string.
	q := url.Values{}
	apollo_common.AddQueryString(q, "q_keywords", "q_keywords", inputs)
	apollo_common.AddQueryString(q, "emailer_campaign_id", "emailer_campaign_id", inputs)
	apollo_common.AddQueryString(q, "contact_id", "contact_id", inputs)
	apollo_common.AddQueryInt(q, "page", "page", inputs)
	apollo_common.AddQueryInt(q, "per_page", "per_page", inputs)

	extra, err := apollo_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	apollo_common.AddQueryFromMap(q, extra)

	resp, err := apollo_common.NewClient(apiKey).PostQuery(flow, "/emailer_messages/search", q)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	msgs := apollo_common.Arr(resp, "emailer_messages")
	return apollo_common.ListResult(msgs, fmt.Sprintf("Found %d email(s)", len(msgs))), nil
}

package marketing_sendgrid_contact_search

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Search Contacts"
	Description  = "Search your marketing contacts with a SendGrid query (SGQL), e.g. email LIKE 'jane%' AND CONTAINS(list_ids, 'your-list-id'). SendGrid returns at most the first 50 matches; the count output reports the total number matched. Emails are stored lower-case, so compare against lower-case addresses."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+magnifying-glass"
	Date         = "09/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "SendGrid API key (SendGrid → Settings → API Keys), e.g. ${secrets.sendgrid_api}", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "Global", Value: ""},
			{Name: "EU (data residency)", Value: "eu"},
		},
		Placeholder: "Global unless your account uses an EU regional subuser — the EU host has no Marketing endpoints (contacts, lists, segments)",
	},
	{Name: "query", Type: core.ConnectionTypeText, Label: "Query (SGQL)", Placeholder: "email LIKE 'jane%' AND CONTAINS(list_ids, 'your-list-id')", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Contacts"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Total Matched"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	query, err := sendgrid.RequiredString("query", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	// The query is raw SGQL, passed through untouched — SendGrid validates it.
	result, _, _, err := sendgrid.Do(auth, http.MethodPost, "/v3/marketing/contacts/search", nil, map[string]interface{}{"query": query})
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid search response shape"), nil
	}
	items, _ := obj["result"].([]interface{})
	count := len(items)
	if total, ok := obj["contact_count"].(float64); ok {
		count = int(total)
	}
	summary := fmt.Sprintf("Found %d matching contact(s)", count)
	if count > len(items) {
		summary = fmt.Sprintf("Found %d matching contact(s) — returning the first %d", count, len(items))
	}
	return sendgrid.ListResult(items, count, summary), nil
}

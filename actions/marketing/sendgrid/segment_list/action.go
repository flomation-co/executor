package marketing_sendgrid_segment_list

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: List Segments"
	Description  = "List the segments in your SendGrid Marketing account — the dynamic groups of contacts defined by a query. Optionally only return segments attached to specific parent lists."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+list"
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
	{Name: "parent_list_ids", Type: core.ConnectionTypeString, Label: "Parent Lists", Placeholder: "Comma-separated list IDs — only return segments attached to these lists"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Segments"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	if ids := sendgrid.SplitCSV(sendgrid.OptionalString("parent_list_ids", inputs)); ids != nil {
		q.Set("parent_list_ids", strings.Join(ids, ","))
	}

	// The segments 2.0 endpoint is unpaginated — every segment comes back in
	// one response — so there is no limit/return_all pair here.
	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/marketing/segments/2.0", q, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid response shape"), nil
	}
	items, _ := obj["results"].([]interface{})
	return sendgrid.ListResult(items, len(items), fmt.Sprintf("Retrieved %d segment(s)", len(items))), nil
}

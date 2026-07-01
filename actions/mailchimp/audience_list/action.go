package mailchimp_audience_list

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	mailchimp "flomation.app/automate/executor/actions/mailchimp"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Audience: List"
	Description  = "List the Mailchimp audiences (lists) in the account. Use this to discover audience (list) IDs for the other actions."
	Website      = "https://www.flomation.co"
	Icon         = "mailchimp+list"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Mailchimp API Key", Placeholder: "xxxxxxxx-us6", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "500 (max 1000)"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Offset"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Audiences"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_items", Type: core.ConnectionTypeInteger, Label: "Total Items"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := mailchimp.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	const path = "/lists"
	const prop = "lists"
	q := url.Values{}

	if mailchimp.OptionalBool("return_all", inputs) {
		items, raw, err := mailchimp.ListAll(apiKey, path, q, prop)
		if err != nil {
			return mailchimp.ErrorResult(err.Error()), nil
		}
		total := 0
		if t, ok := raw["total_items"].(float64); ok {
			total = int(t)
		}
		return mailchimp.ListResult(items, total, raw, fmt.Sprintf("Found %d audience(s)", len(items))), nil
	}

	count := 500
	if n, ok := mailchimp.OptionalInt("limit", inputs); ok && n > 0 {
		count = n
	}
	if count > mailchimp.DefaultPageSize {
		count = mailchimp.DefaultPageSize
	}
	q.Set("count", strconv.Itoa(count))
	if off, ok := mailchimp.OptionalInt("offset", inputs); ok && off > 0 {
		q.Set("offset", strconv.Itoa(off))
	}

	raw, err := mailchimp.Request(apiKey, http.MethodGet, path+"?"+q.Encode(), nil)
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}
	items, _ := raw[prop].([]interface{})
	total := 0
	if t, ok := raw["total_items"].(float64); ok {
		total = int(t)
	}
	return mailchimp.ListResult(items, total, raw, fmt.Sprintf("Found %d audience(s)", len(items))), nil
}

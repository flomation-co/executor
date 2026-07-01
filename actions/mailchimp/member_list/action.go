package mailchimp_member_list

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
	Name         = "Member: List"
	Description  = "List members (subscribers) in a Mailchimp audience. Filter by status, email type, and change/creation timestamps. Returns all matching members or a single page."
	Website      = "https://www.flomation.co"
	Icon         = "mailchimp+magnifying-glass"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Mailchimp API Key", Placeholder: "xxxxxxxx-us6", Required: true},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "Audience (List) ID", Placeholder: "Use Audience: List to find it", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{
		{Name: "Subscribed", Value: "subscribed"},
		{Name: "Unsubscribed", Value: "unsubscribed"},
		{Name: "Cleaned", Value: "cleaned"},
		{Name: "Pending", Value: "pending"},
		{Name: "Transactional", Value: "transactional"},
	}},
	{Name: "email_type", Type: core.ConnectionTypeString, Label: "Email Type", Options: []core.ConnectionOption{
		{Name: "HTML", Value: "html"},
		{Name: "Text", Value: "text"},
	}},
	{Name: "since_last_changed", Type: core.ConnectionTypeString, Label: "Since Last Changed", Placeholder: "ISO timestamp"},
	{Name: "before_last_changed", Type: core.ConnectionTypeString, Label: "Before Last Changed", Placeholder: "ISO timestamp"},
	{Name: "since_timestamp_opt", Type: core.ConnectionTypeString, Label: "Since Opt-in Timestamp", Placeholder: "ISO timestamp"},
	{Name: "before_timestamp_opt", Type: core.ConnectionTypeString, Label: "Before Opt-in Timestamp", Placeholder: "ISO timestamp"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max results (default 500, max 1000)"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Offset"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Members"},
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
	listID, err := mailchimp.RequiredString("list_id", inputs)
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	if v := mailchimp.OptionalString("status", inputs); v != "" {
		q.Set("status", v)
	}
	if v := mailchimp.OptionalString("email_type", inputs); v != "" {
		q.Set("email_type", v)
	}
	if v := mailchimp.OptionalString("since_last_changed", inputs); v != "" {
		q.Set("since_last_changed", v)
	}
	if v := mailchimp.OptionalString("before_last_changed", inputs); v != "" {
		q.Set("before_last_changed", v)
	}
	if v := mailchimp.OptionalString("since_timestamp_opt", inputs); v != "" {
		q.Set("since_timestamp_opt", v)
	}
	if v := mailchimp.OptionalString("before_timestamp_opt", inputs); v != "" {
		q.Set("before_timestamp_opt", v)
	}

	path := mailchimp.MembersPath(listID)

	if mailchimp.OptionalBool("return_all", inputs) {
		items, raw, err := mailchimp.ListAll(apiKey, path, q, "members")
		if err != nil {
			return mailchimp.ErrorResult(err.Error()), nil
		}
		total := 0
		if t, ok := raw["total_items"].(float64); ok {
			total = int(t)
		}
		return mailchimp.ListResult(items, total, raw, fmt.Sprintf("Found %d members", len(items))), nil
	}

	count := 500
	if n, ok := mailchimp.OptionalInt("limit", inputs); ok && n > 0 {
		count = n
	}
	if count > 1000 {
		count = 1000
	}
	q.Set("count", strconv.Itoa(count))
	if off, ok := mailchimp.OptionalInt("offset", inputs); ok && off > 0 {
		q.Set("offset", strconv.Itoa(off))
	}

	raw, err := mailchimp.Request(apiKey, http.MethodGet, path+"?"+q.Encode(), nil)
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}
	items, _ := raw["members"].([]interface{})
	total := 0
	if t, ok := raw["total_items"].(float64); ok {
		total = int(t)
	}
	return mailchimp.ListResult(items, total, raw, fmt.Sprintf("Found %d members", len(items))), nil
}

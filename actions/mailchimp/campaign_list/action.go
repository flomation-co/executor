package mailchimp_campaign_list

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
	Name         = "Campaign: List"
	Description  = "List Mailchimp campaigns, optionally filtered by audience, status, and send/create time. Sort and paginate the results."
	Website      = "https://www.flomation.co"
	Icon         = "mailchimp+magnifying-glass"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Mailchimp API Key", Placeholder: "xxxxxxxx-us6", Required: true},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "Filter by Audience (List) ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{
		{Name: "Save", Value: "save"},
		{Name: "Sending", Value: "sending"},
		{Name: "Schedule", Value: "schedule"},
		{Name: "Sent", Value: "sent"},
	}},
	{Name: "sort_field", Type: core.ConnectionTypeString, Label: "Sort Field", Options: []core.ConnectionOption{
		{Name: "Create Time", Value: "create_time"},
		{Name: "Send Time", Value: "send_time"},
	}},
	{Name: "sort_direction", Type: core.ConnectionTypeString, Label: "Sort Direction", Options: []core.ConnectionOption{
		{Name: "Ascending", Value: "ASC"},
		{Name: "Descending", Value: "DESC"},
	}},
	{Name: "before_send_time", Type: core.ConnectionTypeString, Label: "Before Send Time", Placeholder: "ISO 8601, e.g. 2026-07-01T00:00:00+00:00"},
	{Name: "since_send_time", Type: core.ConnectionTypeString, Label: "Since Send Time", Placeholder: "ISO 8601, e.g. 2026-07-01T00:00:00+00:00"},
	{Name: "before_create_time", Type: core.ConnectionTypeString, Label: "Before Create Time", Placeholder: "ISO 8601, e.g. 2026-07-01T00:00:00+00:00"},
	{Name: "since_create_time", Type: core.ConnectionTypeString, Label: "Since Create Time", Placeholder: "ISO 8601, e.g. 2026-07-01T00:00:00+00:00"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to include, e.g. campaigns.id,campaigns.settings.title"},
	{Name: "exclude_fields", Type: core.ConnectionTypeString, Label: "Exclude Fields", Placeholder: "Comma-separated fields to omit"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "10 (max 1000)"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Offset"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Campaigns"},
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

	const path = "/campaigns"
	const prop = "campaigns"

	q := url.Values{}
	if v := mailchimp.OptionalString("list_id", inputs); v != "" {
		q.Set("list_id", v)
	}
	if v := mailchimp.OptionalString("status", inputs); v != "" {
		q.Set("status", v)
	}
	if v := mailchimp.OptionalString("sort_field", inputs); v != "" {
		q.Set("sort_field", v)
	}
	if v := mailchimp.OptionalString("sort_direction", inputs); v != "" {
		// Mailchimp's query param is "sort_dir", not "sort_direction".
		q.Set("sort_dir", v)
	}
	if v := mailchimp.OptionalString("before_send_time", inputs); v != "" {
		q.Set("before_send_time", v)
	}
	if v := mailchimp.OptionalString("since_send_time", inputs); v != "" {
		q.Set("since_send_time", v)
	}
	if v := mailchimp.OptionalString("before_create_time", inputs); v != "" {
		q.Set("before_create_time", v)
	}
	if v := mailchimp.OptionalString("since_create_time", inputs); v != "" {
		q.Set("since_create_time", v)
	}
	if v := mailchimp.OptionalString("fields", inputs); v != "" {
		q.Set("fields", v)
	}
	if v := mailchimp.OptionalString("exclude_fields", inputs); v != "" {
		q.Set("exclude_fields", v)
	}

	if mailchimp.OptionalBool("return_all", inputs) {
		items, raw, err := mailchimp.ListAll(apiKey, path, q, prop)
		if err != nil {
			return mailchimp.ErrorResult(err.Error()), nil
		}
		total := 0
		if t, ok := raw["total_items"].(float64); ok {
			total = int(t)
		}
		return mailchimp.ListResult(items, total, raw, fmt.Sprintf("Found %d campaigns", len(items))), nil
	}

	count := 10
	if n, ok := mailchimp.OptionalInt("limit", inputs); ok && n > 0 {
		count = n
	}
	if count > mailchimp.DefaultPageSize {
		count = mailchimp.DefaultPageSize // Mailchimp caps count at 1000
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
	return mailchimp.ListResult(items, total, raw, fmt.Sprintf("Found %d campaigns", len(items))), nil
}

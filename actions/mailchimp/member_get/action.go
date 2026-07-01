package mailchimp_member_get

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	mailchimp "flomation.app/automate/executor/actions/mailchimp"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Member: Get"
	Description  = "Retrieve a single member (subscriber) from a Mailchimp audience by email. Optionally include or exclude specific fields. Returns the member."
	Website      = "https://www.flomation.co"
	Icon         = "mailchimp+eye"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Mailchimp API Key", Placeholder: "xxxxxxxx-us6", Required: true},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "Audience (List) ID", Placeholder: "Use Audience: List to find it", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "name@example.com", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to include"},
	{Name: "exclude_fields", Type: core.ConnectionTypeString, Label: "Exclude Fields", Placeholder: "Comma-separated fields to exclude"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Member ID (subscriber hash)"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "member", Type: core.ConnectionTypeObject, Label: "Member"},
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
	email, err := mailchimp.RequiredString("email", inputs)
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	if f := mailchimp.OptionalString("fields", inputs); f != "" {
		q.Set("fields", f)
	}
	if x := mailchimp.OptionalString("exclude_fields", inputs); x != "" {
		q.Set("exclude_fields", x)
	}

	path := mailchimp.MemberPath(listID, email)
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}

	m, err := mailchimp.Request(apiKey, http.MethodGet, path, nil)
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}
	return mailchimp.MemberResult(m, "Retrieved member "+email), nil
}

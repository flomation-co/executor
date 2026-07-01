package mailchimp_member_create

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	mailchimp "flomation.app/automate/executor/actions/mailchimp"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Member: Create"
	Description  = "Add a member (subscriber) to a Mailchimp audience. Set status, merge fields (FNAME/LNAME/…), tags, interests, and location. Returns the created member."
	Website      = "https://www.flomation.co"
	Icon         = "mailchimp+plus"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Mailchimp API Key", Placeholder: "xxxxxxxx-us6", Required: true},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "Audience (List) ID", Placeholder: "Use Audience: List to find it", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "name@example.com", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Required: true, Options: []core.ConnectionOption{
		{Name: "Subscribed", Value: "subscribed"},
		{Name: "Unsubscribed", Value: "unsubscribed"},
		{Name: "Pending (double opt-in)", Value: "pending"},
		{Name: "Cleaned", Value: "cleaned"},
		{Name: "Transactional", Value: "transactional"},
	}},
	{Name: "email_type", Type: core.ConnectionTypeString, Label: "Email Type", Options: []core.ConnectionOption{
		{Name: "HTML", Value: "html"},
		{Name: "Text", Value: "text"},
	}},
	{Name: "language", Type: core.ConnectionTypeString, Label: "Language", Placeholder: "en"},
	{Name: "vip", Type: core.ConnectionTypeBoolean, Label: "VIP"},
	{Name: "ip_signup", Type: core.ConnectionTypeString, Label: "Signup IP", Placeholder: "Opt-in source IP (optional)"},
	{Name: "ip_opt", Type: core.ConnectionTypeString, Label: "Opt-in IP", Placeholder: "Confirmation IP (optional)"},
	{Name: "timestamp_signup", Type: core.ConnectionTypeString, Label: "Signup Timestamp", Placeholder: "ISO 8601, e.g. 2026-07-01T00:00:00+00:00"},
	{Name: "timestamp_opt", Type: core.ConnectionTypeString, Label: "Opt-in Timestamp", Placeholder: "ISO 8601, e.g. 2026-07-01T00:00:00+00:00"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "Comma-separated tag names"},
	{Name: "merge_fields", Type: core.ConnectionTypeKeyValueArray, Label: "Merge Fields", Placeholder: "Merge tag = value, e.g. FNAME = Ada"},
	{Name: "merge_fields_json", Type: core.ConnectionTypeObject, Label: "Merge Fields (JSON, advanced)", Placeholder: `{"FNAME":"Ada","ADDRESS":{...}}`},
	{Name: "interests_json", Type: core.ConnectionTypeObject, Label: "Interests (JSON)", Placeholder: `{"<interestId>":true}`},
	{Name: "location_json", Type: core.ConnectionTypeObject, Label: "Location (JSON)", Placeholder: `{"latitude":51.5,"longitude":-0.1}`},
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
	status, err := mailchimp.RequiredString("status", inputs)
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"email_address": email, "status": status}
	if v := mailchimp.OptionalString("email_type", inputs); v != "" {
		body["email_type"] = v
	}
	if v := mailchimp.OptionalString("language", inputs); v != "" {
		body["language"] = v
	}
	if mailchimp.OptionalBool("vip", inputs) {
		body["vip"] = true
	}
	for _, k := range []string{"ip_signup", "ip_opt", "timestamp_signup", "timestamp_opt"} {
		if v := mailchimp.OptionalString(k, inputs); v != "" {
			body[k] = v
		}
	}
	if tags := mailchimp.CSVToList(mailchimp.OptionalString("tags", inputs)); len(tags) > 0 {
		body["tags"] = tags
	}
	if mf, err := mailchimp.BuildMergeFields(inputs); err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	} else if len(mf) > 0 {
		body["merge_fields"] = mf
	}
	if interests, err := mailchimp.ParseJSONObject("interests_json", inputs); err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	} else if interests != nil {
		body["interests"] = interests
	}
	if location, err := mailchimp.ParseJSONObject("location_json", inputs); err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	} else if location != nil {
		body["location"] = location
	}

	m, err := mailchimp.Request(apiKey, http.MethodPost, mailchimp.MembersPath(listID), body)
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}
	return mailchimp.MemberResult(m, fmt.Sprintf("Added %s to audience %s", email, listID)), nil
}

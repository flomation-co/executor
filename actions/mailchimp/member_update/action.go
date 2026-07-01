package mailchimp_member_update

import (
	"net/http"

	core "flomation.app/automate/executor"
	mailchimp "flomation.app/automate/executor/actions/mailchimp"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Member: Update"
	Description  = "Update a member (subscriber) in a Mailchimp audience, creating it if it does not exist (upsert). Set status, merge fields, interests, location, and more. Returns the member."
	Website      = "https://www.flomation.co"
	Icon         = "mailchimp+pencil"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Mailchimp API Key", Placeholder: "xxxxxxxx-us6", Required: true},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "Audience (List) ID", Placeholder: "Use Audience: List to find it", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "name@example.com", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Required only when creating a new member", Options: []core.ConnectionOption{
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
	{Name: "merge_fields", Type: core.ConnectionTypeKeyValueArray, Label: "Merge Fields", Placeholder: "Merge tag = value, e.g. FNAME = Ada"},
	{Name: "merge_fields_json", Type: core.ConnectionTypeObject, Label: "Merge Fields (JSON, advanced)", Placeholder: `{"FNAME":"Ada","ADDRESS":{...}}`},
	{Name: "interests_json", Type: core.ConnectionTypeObject, Label: "Interests (JSON)", Placeholder: `{"<interestId>":true}`},
	{Name: "location_json", Type: core.ConnectionTypeObject, Label: "Location (JSON)", Placeholder: `{"latitude":51.5,"longitude":-0.1}`},
	{Name: "skip_merge_validation", Type: core.ConnectionTypeBoolean, Label: "Skip Merge Validation"},
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

	body := map[string]interface{}{"email_address": email}
	if v := mailchimp.OptionalString("status", inputs); v != "" {
		body["status"] = v
		// PUT is an upsert: status_if_new sets the status of a member created
		// when the email doesn't exist yet (Mailchimp requires it for that
		// path; status alone only updates an existing member).
		body["status_if_new"] = v
	}
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

	path := mailchimp.MemberPath(listID, email)
	if mailchimp.OptionalBool("skip_merge_validation", inputs) {
		path += "?skip_merge_validation=true"
	}

	m, err := mailchimp.Request(apiKey, http.MethodPut, path, body)
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}
	return mailchimp.MemberResult(m, "Updated member "+email), nil
}

package mailchimp_member_tag_remove

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	mailchimp "flomation.app/automate/executor/actions/mailchimp"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Member Tag: Remove"
	Description  = "Remove one or more tags from a Mailchimp audience member. Tags are matched by name and marked inactive."
	Website      = "https://www.flomation.co"
	Icon         = "mailchimp+minus"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Mailchimp API Key", Placeholder: "xxxxxxxx-us6", Required: true},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "Audience (List) ID", Placeholder: "Use Audience: List to find it", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "name@example.com", Required: true},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "Comma-separated tag names", Required: true},
	{Name: "is_syncing", Type: core.ConnectionTypeBoolean, Label: "Suppress tag-triggered automations"},
}

var Outputs = [...]core.Connection{
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
	tagsInput, err := mailchimp.RequiredString("tags", inputs)
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}

	names := mailchimp.CSVToList(tagsInput)
	tags := make([]interface{}, 0, len(names))
	for _, n := range names {
		tags = append(tags, map[string]interface{}{"name": n, "status": "inactive"})
	}
	body := map[string]interface{}{"tags": tags}
	if mailchimp.OptionalBool("is_syncing", inputs) {
		body["is_syncing"] = true
	}

	if err := mailchimp.RequestNoContent(apiKey, http.MethodPost, mailchimp.MemberPath(listID, email)+"/tags", body); err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}
	return mailchimp.SuccessResult(fmt.Sprintf("Removed %d tag(s) from %s", len(names), email)), nil
}

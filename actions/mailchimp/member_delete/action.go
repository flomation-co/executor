package mailchimp_member_delete

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	mailchimp "flomation.app/automate/executor/actions/mailchimp"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Member: Delete"
	Description  = "Remove a member (subscriber) from a Mailchimp audience. By default archives the member (reversible); optionally permanently delete them (irreversible)."
	Website      = "https://www.flomation.co"
	Icon         = "mailchimp+trash"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Mailchimp API Key", Placeholder: "xxxxxxxx-us6", Required: true},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "Audience (List) ID", Placeholder: "Use Audience: List to find it", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "name@example.com", Required: true},
	{Name: "permanent", Type: core.ConnectionTypeBoolean, Label: "Permanent (irreversible)", Placeholder: "Off = archive (reversible); On = permanently delete"},
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

	verb := "Archived"
	if mailchimp.OptionalBool("permanent", inputs) {
		verb = "Permanently deleted"
		path := mailchimp.MemberPath(listID, email) + "/actions/delete-permanent"
		err = mailchimp.RequestNoContent(apiKey, http.MethodPost, path, nil)
	} else {
		err = mailchimp.RequestNoContent(apiKey, http.MethodDelete, mailchimp.MemberPath(listID, email), nil)
	}
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}
	return mailchimp.SuccessResult(fmt.Sprintf("%s member %s", verb, email)), nil
}

package email_draft_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Email: Create Draft"
	Description  = "Create a one-off email draft to a contact (does not send). Master key required."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+pen"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey} (master key)", Required: true},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Apollo contact to email", Required: true},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Quick question, {{first_name}}"},
	{Name: "body_html", Type: core.ConnectionTypeText, Label: "Body (HTML)", Placeholder: "<p>Hi {{first_name}}…</p>"},
	{Name: "emailer_template_id", Type: core.ConnectionTypeString, Label: "Template ID", Placeholder: "Optional Apollo email template ID"},
	{Name: "enable_tracking", Type: core.ConnectionTypeBoolean, Label: "Enable Tracking", Placeholder: "Track opens/clicks"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON)", Placeholder: `{"recipients":[{"email":"…","type":"cc"}]}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Email Message ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Email Message"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	if _, err := apollo_common.RequiredString("contact_id", inputs); err != nil {
		return apollo_common.ErrorResult("a contact ID is required"), nil
	}

	body := map[string]interface{}{}
	apollo_common.SetString(body, "contact_id", "contact_id", inputs)
	apollo_common.SetString(body, "subject", "subject", inputs)
	apollo_common.SetString(body, "body_html", "body_html", inputs)
	apollo_common.SetString(body, "emailer_template_id", "emailer_template_id", inputs)
	apollo_common.SetBool(body, "enable_tracking", "enable_tracking", inputs)

	extra, err := apollo_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	apollo_common.MergeFields(body, extra)

	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/emailer_messages", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	msg := apollo_common.Obj(resp, "emailer_message")
	if msg == nil {
		return apollo_common.ErrorResult("email draft was not created"), nil
	}
	return apollo_common.ObjectResult("", msg, fmt.Sprintf("Created email draft %s (not sent)", apollo_common.IDOf(msg))), nil
}

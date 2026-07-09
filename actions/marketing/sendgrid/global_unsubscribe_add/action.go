package marketing_sendgrid_global_unsubscribe_add

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Add Global Unsubscribes"
	Description  = "Add one or more email addresses to SendGrid's global unsubscribe list so they stop receiving all email from your account. Separate multiple addresses with commas."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+ban"
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
	{Name: "emails", Type: core.ConnectionTypeString, Label: "Emails", Placeholder: "recipient@example.com — separate multiple addresses with commas", Required: true},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `Any other SendGrid field to include in the request body`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Added Emails"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	emailsRaw, err := sendgrid.RequiredString("emails", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	emails := sendgrid.SplitCSV(emailsRaw)
	if emails == nil {
		return sendgrid.ErrorResult("emails is required"), nil
	}

	body := map[string]interface{}{"recipient_emails": emails}
	if err := sendgrid.MergeAdditionalFields(body, inputs); err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodPost, "/v3/asm/suppressions/global", nil, body)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, _ := result.(map[string]interface{})
	return sendgrid.ResourceResult("", obj, fmt.Sprintf("Added %d email(s) to the global unsubscribe list", len(emails))), nil
}

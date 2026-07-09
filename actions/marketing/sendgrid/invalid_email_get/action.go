package marketing_sendgrid_invalid_email_get

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Get Invalid Email"
	Description  = "Look up a single email address on your SendGrid invalid email list. If the address is not on the list, the action reports that no invalid email was found."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+eye"
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
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "The address to look up, e.g. jane@example.com", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Email"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Invalid Email"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	email, err := sendgrid.RequiredString("email", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/suppression/invalid_emails/"+url.PathEscape(email), nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	// The per-email endpoint answers with a JSON array; empty means the
	// address is not on the invalid email list.
	items, ok := result.([]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid suppression response shape"), nil
	}
	if len(items) == 0 {
		return sendgrid.ErrorResult(fmt.Sprintf("no invalid email found for %s", email)), nil
	}
	obj, _ := items[0].(map[string]interface{})
	return sendgrid.ResourceResult(email, obj, fmt.Sprintf("Retrieved invalid email for %s", email)), nil
}

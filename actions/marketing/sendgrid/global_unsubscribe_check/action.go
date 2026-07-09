package marketing_sendgrid_global_unsubscribe_check

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
	Name         = "SendGrid: Check Global Unsubscribe"
	Description  = "Check whether an email address is on SendGrid's global unsubscribe list. Returns a clear yes or no — an address that is not on the list is a normal result, not an error."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+check"
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
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "recipient@example.com — the address to check", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Email"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Check Result"},
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

	// The API always answers 200: {"recipient_email": ...} when the address is
	// suppressed, an EMPTY object when it is not — a non-suppressed address is
	// a normal result, never an error.
	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/asm/suppressions/global/"+url.PathEscape(email), nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	suppressed := false
	if obj, ok := result.(map[string]interface{}); ok {
		_, suppressed = obj["recipient_email"]
	}
	summary := fmt.Sprintf("%s is not on the global unsubscribe list", email)
	if suppressed {
		summary = fmt.Sprintf("%s is on the global unsubscribe list", email)
	}
	res := map[string]interface{}{"email": email, "suppressed": suppressed}
	return sendgrid.ResourceResult(email, res, summary), nil
}

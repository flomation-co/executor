package marketing_sendgrid_contact_get_by_email

import (
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Get Contact by Email"
	Description  = "Look up a single marketing contact by email address (primary or alternate) and return the full contact record. Note that a contact added moments ago may not be found yet — SendGrid applies contact changes asynchronously."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+envelope"
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
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "jane@example.com — the address to look up", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Contact"},
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
	// SendGrid stores emails lower-cased and keys the response by the searched
	// address, so the lookup is lower-cased up front.
	email = strings.ToLower(email)

	body := map[string]interface{}{"emails": []string{email}}
	result, _, status, err := sendgrid.Do(auth, http.MethodPost, "/v3/marketing/contacts/search/emails", nil, body)
	if err != nil {
		// The endpoint answers 404 when no contact matches — that is "no
		// match", not a broken request.
		if status == http.StatusNotFound {
			return sendgrid.ErrorResult(fmt.Sprintf("no contact found for %s", email)), nil
		}
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid contact response shape"), nil
	}
	byEmail, _ := obj["result"].(map[string]interface{})
	entry, _ := byEmail[email].(map[string]interface{})
	contact, _ := entry["contact"].(map[string]interface{})
	if contact == nil {
		return sendgrid.ErrorResult(fmt.Sprintf("no contact found for %s", email)), nil
	}
	return sendgrid.ResourceResult(sendgrid.StringifyID(contact["id"]), contact, "Retrieved contact "+email), nil
}

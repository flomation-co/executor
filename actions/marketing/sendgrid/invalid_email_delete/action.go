package marketing_sendgrid_invalid_email_delete

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
	Name         = "SendGrid: Delete Invalid Email"
	Description  = "Remove addresses from your SendGrid invalid email list so delivery can be attempted again. Provide a single Email, a comma-separated Emails list, or tick Delete All to clear the entire list."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+trash"
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
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "A single address to remove — leave blank to use Emails or Delete All"},
	{Name: "emails", Type: core.ConnectionTypeString, Label: "Emails", Placeholder: "Comma-separated addresses to remove in one go — cannot be combined with Delete All"},
	{Name: "delete_all", Type: core.ConnectionTypeBoolean, Label: "Delete All", Placeholder: "Tick to clear the entire invalid email list — cannot be combined with Emails"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Email"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	email := sendgrid.OptionalString("email", inputs)
	emails := sendgrid.SplitCSV(sendgrid.OptionalString("emails", inputs))
	deleteAll, _ := sendgrid.OptionalBoolSet("delete_all", inputs)

	switch {
	case email != "":
		if _, _, _, err := sendgrid.Do(auth, http.MethodDelete, "/v3/suppression/invalid_emails/"+url.PathEscape(email), nil, nil); err != nil {
			return sendgrid.ErrorResult(err.Error()), nil
		}
		return sendgrid.SuccessResult(email, fmt.Sprintf("Deleted invalid email for %s", email)), nil
	case emails != nil && deleteAll:
		return sendgrid.ErrorResult("provide Emails or tick Delete All, not both"), nil
	case emails != nil:
		body := map[string]interface{}{"emails": emails}
		if _, _, _, err := sendgrid.Do(auth, http.MethodDelete, "/v3/suppression/invalid_emails", nil, body); err != nil {
			return sendgrid.ErrorResult(err.Error()), nil
		}
		return sendgrid.SuccessResult("", fmt.Sprintf("Deleted %d invalid email(s)", len(emails))), nil
	case deleteAll:
		if _, _, _, err := sendgrid.Do(auth, http.MethodDelete, "/v3/suppression/invalid_emails", nil, map[string]interface{}{"delete_all": true}); err != nil {
			return sendgrid.ErrorResult(err.Error()), nil
		}
		return sendgrid.SuccessResult("", "Deleted all invalid emails"), nil
	default:
		return sendgrid.ErrorResult("provide an Email or Emails to remove, or tick Delete All"), nil
	}
}

package marketing_sendgrid_template_delete

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Delete Template"
	Description  = "Permanently delete a transactional template from SendGrid, including all of its versions. This cannot be undone."
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
	{Name: "template_id", Type: core.ConnectionTypeString, Label: "Template", Placeholder: "The template to delete — see \"SendGrid: List Templates\"", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Template ID"},
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

	templateID, err := sendgrid.RequiredString("template_id", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	_, _, _, err = sendgrid.Do(auth, http.MethodDelete, "/v3/templates/"+url.PathEscape(templateID), nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	return sendgrid.SuccessResult(templateID, "Deleted template "+templateID), nil
}

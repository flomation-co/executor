package marketing_sendgrid_template_get

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Get Template"
	Description  = "Look up a single transactional template in SendGrid, including every version of the template."
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
	{Name: "template_id", Type: core.ConnectionTypeString, Label: "Template", Placeholder: "The template to retrieve — see \"SendGrid: List Templates\"", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Template ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Template"},
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

	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/templates/"+url.PathEscape(templateID), nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, _ := result.(map[string]interface{})
	id := sendgrid.StringifyID(obj["id"])
	if id == "" {
		id = templateID
	}
	label := id
	if name, _ := obj["name"].(string); name != "" {
		label = name
	}
	return sendgrid.ResourceResult(id, obj, "Retrieved template "+label), nil
}

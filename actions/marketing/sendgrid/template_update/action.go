package marketing_sendgrid_template_update

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
	Name         = "SendGrid: Update Template"
	Description  = "Rename a transactional template in SendGrid. The template's versions and content are unchanged."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+pencil"
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
	{Name: "template_id", Type: core.ConnectionTypeString, Label: "Template", Placeholder: "The template to rename — see \"SendGrid: List Templates\"", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The new name for the template", Required: true},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `Any other SendGrid template field to update`},
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
	name, err := sendgrid.RequiredString("name", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{"name": name}
	if err := sendgrid.MergeAdditionalFields(body, inputs); err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodPatch, "/v3/templates/"+url.PathEscape(templateID), nil, body)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, _ := result.(map[string]interface{})
	id := sendgrid.StringifyID(obj["id"])
	if id == "" {
		id = templateID
	}
	return sendgrid.ResourceResult(id, obj, fmt.Sprintf("Renamed template to %s", name)), nil
}

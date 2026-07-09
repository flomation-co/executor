package marketing_sendgrid_template_version_activate

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
	Name         = "SendGrid: Activate Template Version"
	Description  = "Make a specific version of a SendGrid transactional template the active one — the version that is used whenever the template is sent."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+play"
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
	{Name: "template_id", Type: core.ConnectionTypeString, Label: "Template", Placeholder: "The template the version belongs to — see \"SendGrid: List Templates\"", Required: true},
	{Name: "version_id", Type: core.ConnectionTypeString, Label: "Version ID", Placeholder: "The version to activate — see \"SendGrid: Get Template\" for the template's versions", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Version ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Template Version"},
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
	versionID, err := sendgrid.RequiredString("version_id", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	// Activation is a POST with an EMPTY body (not a PATCH).
	path := "/v3/templates/" + url.PathEscape(templateID) + "/versions/" + url.PathEscape(versionID) + "/activate"
	result, _, _, err := sendgrid.Do(auth, http.MethodPost, path, nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, _ := result.(map[string]interface{})
	id := sendgrid.StringifyID(obj["id"])
	if id == "" {
		id = versionID
	}
	return sendgrid.ResourceResult(id, obj, fmt.Sprintf("Activated version %s of template %s", id, templateID)), nil
}

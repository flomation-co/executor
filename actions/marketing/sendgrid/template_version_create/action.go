package marketing_sendgrid_template_version_create

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
	Name         = "SendGrid: Create Template Version"
	Description  = "Add a new version to a SendGrid transactional template. Provide the subject and HTML content, and tick Active to make this the version that is sent."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+plus"
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
	{Name: "template_id", Type: core.ConnectionTypeString, Label: "Template", Placeholder: "The template to add a version to — see \"SendGrid: List Templates\"", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "A name for this version, e.g. July redesign", Required: true},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "The email subject line — {{handlebars}} are substituted at send time"},
	{Name: "html_content", Type: core.ConnectionTypeText, Label: "HTML Content", Placeholder: "The HTML body of the email — {{handlebars}} are substituted at send time"},
	{Name: "plain_content", Type: core.ConnectionTypeText, Label: "Plain Text Content", Placeholder: "The plain-text body — leave blank to have SendGrid generate it from the HTML"},
	{Name: "generate_plain_content", Type: core.ConnectionTypeBoolean, Label: "Generate Plain Text", Placeholder: "Tick to have SendGrid regenerate the plain-text body from the HTML content"},
	{Name: "active", Type: core.ConnectionTypeBoolean, Label: "Active", Placeholder: "Tick to make this the active version — the one used when the template is sent"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `Any other SendGrid version field, e.g. {"test_data":"{\"first_name\":\"Jane\"}"}`},
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
	name, err := sendgrid.RequiredString("name", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{"name": name}
	sendgrid.SetIfPresent(body, inputs, "subject", "subject")
	sendgrid.SetIfPresent(body, inputs, "html_content", "html_content")
	sendgrid.SetIfPresent(body, inputs, "plain_content", "plain_content")
	sendgrid.SetBoolIfSet(body, inputs, "generate_plain_content", "generate_plain_content")
	// SendGrid defaults generate_plain_content to TRUE, which would silently
	// overwrite a user-supplied plain_content with text derived from the HTML —
	// so when the checkbox is untouched but plain text was provided, send false.
	if _, set := sendgrid.OptionalBoolSet("generate_plain_content", inputs); !set {
		if _, hasPlain := body["plain_content"]; hasPlain {
			body["generate_plain_content"] = false
		}
	}
	// The API takes active as an integer flag: 1 makes this the active version.
	active, activeSet := sendgrid.OptionalBoolSet("active", inputs)
	if activeSet {
		n := 0
		if active {
			n = 1
		}
		body["active"] = n
	}
	if err := sendgrid.MergeAdditionalFields(body, inputs); err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodPost, "/v3/templates/"+url.PathEscape(templateID)+"/versions", nil, body)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, _ := result.(map[string]interface{})
	summary := fmt.Sprintf("Created version %s for template %s", name, templateID)
	if activeSet && active {
		summary += " and made it active"
	}
	return sendgrid.ResourceResult(sendgrid.StringifyID(obj["id"]), obj, summary), nil
}

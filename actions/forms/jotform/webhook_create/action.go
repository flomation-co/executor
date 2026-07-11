// Package webhook_create registers a JotForm webhook for a form.
package webhook_create

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/jotform"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Webhook"
	Description  = "Register a JotForm webhook that fires on each new submission of a form."
	Website      = "https://www.flomation.co"
	Icon         = "webhook+plus"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "JotForm API Key", Placeholder: "${secrets.jotform_api_key}", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "us", Options: []core.ConnectionOption{
		{Name: "US (default)", Value: "us"},
		{Name: "EU", Value: "eu"},
		{Name: "HIPAA", Value: "hipaa"},
	}},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID", Placeholder: "231234567890", Required: true},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Webhook URL", Placeholder: "https://launch.flomation.app/webhook/…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Webhooks"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := jotform.Get(inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	region := jotform.Region(inputs)
	formID, err := forms_common.RequiredString("form_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	webhookURL, err := forms_common.RequiredString("url", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	// JotForm expects the webhook target as a form-encoded parameter, not JSON.
	form := url.Values{}
	form.Set("webhookURL", webhookURL)

	raw, status, err := jotform.DoForm(jotform.Context(flow), http.MethodPost, "/form/"+formID+"/webhooks", apiKey, region, form)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("JotForm request failed: %v", err)), nil
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return forms_common.ErrorResult(jotform.StatusMessage(status, raw)), nil
	}

	// content is the map of registered webhook slots (index → URL).
	content := jotform.Content(raw)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Registered JotForm webhook on form %s → %s.", formID, webhookURL),
		"form_id":     formID,
		"result":      content,
		"success":     true,
		"error":       "",
	}, nil
}

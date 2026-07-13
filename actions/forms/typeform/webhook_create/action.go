// Package webhook_create registers or updates a Typeform webhook for a form.
package webhook_create

import (
	"encoding/json"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/typeform"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Webhook"
	Description  = "Register or update a Typeform webhook that fires on each form submission."
	Website      = "https://www.flomation.co"
	Icon         = "webhook+plus"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Typeform Personal Access Token", Placeholder: "${secrets.typeform_token}", Required: true},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID", Placeholder: "abc123", Required: true},
	{Name: "tag", Type: core.ConnectionTypeString, Label: "Tag", Placeholder: "flomation", Required: true},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Webhook URL", Placeholder: "https://launch.flomation.app/webhook/…", Required: true},
	{Name: "secret", Type: core.ConnectionTypeSecret, Label: "Signing Secret", Placeholder: "${secrets.typeform_webhook_secret}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "webhook_id", Type: core.ConnectionTypeString, Label: "Webhook ID"},
	{Name: "tag", Type: core.ConnectionTypeString, Label: "Tag"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Webhook"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := typeform.Get(inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	formID, err := forms_common.RequiredString("form_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	tag, err := forms_common.RequiredString("tag", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	webhookURL, err := forms_common.RequiredString("url", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	payload := map[string]interface{}{
		"url":        webhookURL,
		"enabled":    true,
		"verify_ssl": true,
	}
	if secret := forms_common.OptionalString("secret", inputs); secret != "" {
		payload["secret"] = secret
	}

	body, _ := json.Marshal(payload)
	obj, status, err := typeform.Do(typeform.Context(flow), http.MethodPut, "/forms/"+formID+"/webhooks/"+tag, token, body)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Typeform request failed: %v", err)), nil
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return forms_common.ErrorResult(typeform.StatusMessage(status, obj)), nil
	}

	webhookID, _ := obj["id"].(string)
	result := forms_common.ObjectResult(obj, fmt.Sprintf("Registered Typeform webhook %q on form %s → %s.", tag, formID, webhookURL))
	result["webhook_id"] = webhookID
	result["tag"] = tag
	return result, nil
}

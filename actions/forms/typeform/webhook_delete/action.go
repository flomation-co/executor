// Package webhook_delete removes a Typeform webhook from a form.
package webhook_delete

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/typeform"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Delete Webhook"
	Description  = "Delete a Typeform webhook from a form by its tag."
	Website      = "https://www.flomation.co"
	Icon         = "webhook+trash"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Typeform Personal Access Token", Placeholder: "${secrets.typeform_token}", Required: true},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID", Placeholder: "abc123", Required: true},
	{Name: "tag", Type: core.ConnectionTypeString, Label: "Tag", Placeholder: "flomation", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "tag", Type: core.ConnectionTypeString, Label: "Tag"},
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

	obj, status, err := typeform.Do(typeform.Context(flow), http.MethodDelete, "/forms/"+formID+"/webhooks/"+tag, token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Typeform request failed: %v", err)), nil
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return forms_common.ErrorResult(typeform.StatusMessage(status, obj)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted Typeform webhook %q from form %s.", tag, formID),
		"tag":         tag,
		"success":     true,
		"error":       "",
	}, nil
}

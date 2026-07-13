// Package form_delete deletes a Typeform form by ID.
package form_delete

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
	Name         = "Delete Form"
	Description  = "Delete a Typeform form permanently by its form ID."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+trash"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Typeform Personal Access Token", Placeholder: "${secrets.typeform_token}", Required: true},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID", Placeholder: "abc123", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID"},
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

	obj, status, err := typeform.Do(typeform.Context(flow), http.MethodDelete, "/forms/"+formID, token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Typeform request failed: %v", err)), nil
	}
	// Typeform returns 204 No Content on a successful delete.
	if status != http.StatusOK && status != http.StatusNoContent {
		return forms_common.ErrorResult(typeform.StatusMessage(status, obj)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted Typeform form %s.", formID),
		"form_id":     formID,
		"success":     true,
		"error":       "",
	}, nil
}

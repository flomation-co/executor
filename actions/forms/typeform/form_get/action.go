// Package form_get retrieves a single Typeform form by ID.
package form_get

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
	Name         = "Get Form"
	Description  = "Retrieve a single Typeform form definition by its form ID."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+eye"
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
	{Name: "form_url", Type: core.ConnectionTypeString, Label: "Form URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Form"},
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

	obj, status, err := typeform.Do(typeform.Context(flow), http.MethodGet, "/forms/"+formID, token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Typeform request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(typeform.StatusMessage(status, obj)), nil
	}

	title, _ := obj["title"].(string)
	result := forms_common.ObjectResult(obj, fmt.Sprintf("Retrieved Typeform form %q (%s).", title, formID))
	result["form_id"] = formID
	if links, ok := obj["_links"].(map[string]interface{}); ok {
		if u, ok := links["display"].(string); ok {
			result["form_url"] = u
		}
	}
	if _, ok := result["form_url"]; !ok {
		result["form_url"] = ""
	}
	return result, nil
}

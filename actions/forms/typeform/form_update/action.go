// Package form_update replaces a Typeform form definition.
package form_update

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
	Name         = "Update Form"
	Description  = "Replace a Typeform form definition with a full form JSON object."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+file-pen"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Typeform Personal Access Token", Placeholder: "${secrets.typeform_token}", Required: true},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID", Placeholder: "abc123", Required: true},
	{Name: "form", Type: core.ConnectionTypeString, Label: "Form (JSON object)", Placeholder: `{"title":"Updated title","fields":[...]}`, Required: true},
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
	rawForm, err := forms_common.RequiredString("form", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	var formObj map[string]interface{}
	if err := json.Unmarshal([]byte(rawForm), &formObj); err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Invalid form JSON: %v", err)), nil
	}

	body, _ := json.Marshal(formObj)
	obj, status, err := typeform.Do(typeform.Context(flow), http.MethodPut, "/forms/"+formID, token, body)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Typeform request failed: %v", err)), nil
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return forms_common.ErrorResult(typeform.StatusMessage(status, obj)), nil
	}

	title, _ := obj["title"].(string)
	result := forms_common.ObjectResult(obj, fmt.Sprintf("Updated Typeform form %q (%s).", title, formID))
	result["form_id"] = formID
	result["form_url"] = ""
	if links, ok := obj["_links"].(map[string]interface{}); ok {
		if u, ok := links["display"].(string); ok {
			result["form_url"] = u
		}
	}
	return result, nil
}

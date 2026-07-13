// Package form_get retrieves a Google Form's definition and metadata.
package form_get

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/googleforms"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Form"
	Description  = "Retrieve a Google Form's definition, items and metadata by form ID."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+eye"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID", Placeholder: "1FAIpQL...", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID"},
	{Name: "responder_uri", Type: core.ConnectionTypeString, Label: "Responder URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Form"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	formID, err := forms_common.RequiredString("form_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	token, err := googleforms.Token(flow, inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	obj, status, err := googleforms.Do(flow, http.MethodGet, "/forms/"+formID, token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Google Forms request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(googleforms.StatusMessage(status, obj)), nil
	}

	title := formTitle(obj)
	responderURI, _ := obj["responderUri"].(string)
	result := forms_common.ObjectResult(obj, fmt.Sprintf("Retrieved Google Form %q (%s).", title, formID))
	result["form_id"] = formID
	result["responder_uri"] = responderURI
	return result, nil
}

// formTitle extracts info.title from a form object, falling back to "".
func formTitle(obj map[string]interface{}) string {
	info, ok := obj["info"].(map[string]interface{})
	if !ok {
		return ""
	}
	title, _ := info["title"].(string)
	return title
}

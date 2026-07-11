// Package form_create creates a new Google Form.
package form_create

import (
	"encoding/json"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/googleforms"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Form"
	Description  = "Create a new Google Form with a title using a connected Google account."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+plus"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "Customer feedback", Required: true},
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
	title, err := forms_common.RequiredString("title", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	token, err := googleforms.Token(flow, inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	// Google Forms only permits "info.title"/"info.documentTitle" at create
	// time; questions must be added afterwards via batchUpdate.
	payload := map[string]interface{}{
		"info": map[string]interface{}{
			"title":         title,
			"documentTitle": title,
		},
	}
	body, _ := json.Marshal(payload)

	obj, status, err := googleforms.Do(flow, http.MethodPost, "/forms", token, body)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Google Forms request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(googleforms.StatusMessage(status, obj)), nil
	}

	formID, _ := obj["formId"].(string)
	responderURI, _ := obj["responderUri"].(string)
	result := forms_common.ObjectResult(obj, fmt.Sprintf("Created Google Form %q (%s).", title, formID))
	result["form_id"] = formID
	result["responder_uri"] = responderURI
	return result, nil
}

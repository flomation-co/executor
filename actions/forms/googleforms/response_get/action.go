// Package response_get retrieves a single Google Form response by ID.
package response_get

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
	Name         = "Get Response"
	Description  = "Retrieve a single submitted response from a Google Form by response ID."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+magnifying-glass"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID", Placeholder: "1FAIpQL...", Required: true},
	{Name: "response_id", Type: core.ConnectionTypeString, Label: "Response ID", Placeholder: "ACYDBNh...", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "response_id", Type: core.ConnectionTypeString, Label: "Response ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Response"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	formID, err := forms_common.RequiredString("form_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	responseID, err := forms_common.RequiredString("response_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	token, err := googleforms.Token(flow, inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	obj, status, err := googleforms.Do(flow, http.MethodGet, "/forms/"+formID+"/responses/"+responseID, token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Google Forms request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(googleforms.StatusMessage(status, obj)), nil
	}

	rid, _ := obj["responseId"].(string)
	if rid == "" {
		rid = responseID
	}
	result := forms_common.ObjectResult(obj, fmt.Sprintf("Retrieved response %s for Google Form %s.", rid, formID))
	result["response_id"] = rid
	return result, nil
}

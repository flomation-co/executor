// Package form_batch_update applies a batchUpdate to a Google Form, typically to
// add items or questions.
package form_batch_update

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
	Name         = "Batch Update Form"
	Description  = "Add items or questions to a Google Form via a batchUpdate requests JSON array."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+pencil"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID", Placeholder: "1FAIpQL...", Required: true},
	{Name: "requests", Type: core.ConnectionTypeString, Label: "Requests (JSON array)", Placeholder: `[{"createItem":{"item":{"title":"Your name","questionItem":{"question":{"required":true,"textQuestion":{}}}},"location":{"index":0}}}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Reply"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	formID, err := forms_common.RequiredString("form_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	rawRequests, err := forms_common.RequiredString("requests", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	var requests []interface{}
	if err := json.Unmarshal([]byte(rawRequests), &requests); err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Invalid requests JSON: %v", err)), nil
	}

	token, err := googleforms.Token(flow, inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	body, _ := json.Marshal(map[string]interface{}{"requests": requests})
	obj, status, err := googleforms.Do(flow, http.MethodPost, "/forms/"+formID+":batchUpdate", token, body)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Google Forms request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(googleforms.StatusMessage(status, obj)), nil
	}

	result := forms_common.ObjectResult(obj, fmt.Sprintf("Applied %d batchUpdate request(s) to Google Form %s.", len(requests), formID))
	result["form_id"] = formID
	return result, nil
}

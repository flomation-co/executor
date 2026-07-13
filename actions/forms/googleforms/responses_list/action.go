// Package responses_list retrieves the submitted responses for a Google Form.
package responses_list

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
	Name         = "List Responses"
	Description  = "Retrieve the submitted responses for a Google Form by form ID."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+envelope-open-text"
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
	{Name: "responses", Type: core.ConnectionTypeObject, Label: "Responses"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
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

	obj, status, err := googleforms.Do(flow, http.MethodGet, "/forms/"+formID+"/responses", token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Google Forms request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(googleforms.StatusMessage(status, obj)), nil
	}

	responses := make([]map[string]interface{}, 0)
	if rawItems, ok := obj["responses"].([]interface{}); ok {
		for _, it := range rawItems {
			if m, ok := it.(map[string]interface{}); ok {
				responses = append(responses, m)
			}
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Retrieved %d response(s) for Google Form %s.", len(responses), formID),
		"responses":   responses,
		"total":       len(responses),
		"success":     true,
		"error":       "",
	}, nil
}

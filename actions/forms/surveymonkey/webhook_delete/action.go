// Package webhook_delete removes a SurveyMonkey webhook by ID.
package webhook_delete

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/surveymonkey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Delete Webhook"
	Description  = "Delete a SurveyMonkey webhook by its webhook ID."
	Website      = "https://www.flomation.co"
	Icon         = "webhook+trash"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "SurveyMonkey Access Token", Placeholder: "${secrets.surveymonkey_token}", Required: true},
	{Name: "webhook_id", Type: core.ConnectionTypeString, Label: "Webhook ID", Placeholder: "12345", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "webhook_id", Type: core.ConnectionTypeString, Label: "Webhook ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := surveymonkey.Get(inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	webhookID, err := forms_common.RequiredString("webhook_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	obj, status, err := surveymonkey.Do(surveymonkey.Context(flow), http.MethodDelete, "/webhooks/"+webhookID, token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("SurveyMonkey request failed: %v", err)), nil
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return forms_common.ErrorResult(surveymonkey.StatusMessage(status, obj)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted SurveyMonkey webhook %s.", webhookID),
		"webhook_id":  webhookID,
		"success":     true,
		"error":       "",
	}, nil
}

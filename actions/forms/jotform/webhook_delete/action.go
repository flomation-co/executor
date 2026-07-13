// Package webhook_delete removes a JotForm webhook from a form.
package webhook_delete

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/jotform"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Delete Webhook"
	Description  = "Remove a JotForm webhook from a form by its webhook ID (index)."
	Website      = "https://www.flomation.co"
	Icon         = "webhook+trash"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "JotForm API Key", Placeholder: "${secrets.jotform_api_key}", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "us", Options: []core.ConnectionOption{
		{Name: "US (default)", Value: "us"},
		{Name: "EU", Value: "eu"},
		{Name: "HIPAA", Value: "hipaa"},
	}},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID", Placeholder: "231234567890", Required: true},
	{Name: "webhook_id", Type: core.ConnectionTypeString, Label: "Webhook ID", Placeholder: "0", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID"},
	{Name: "webhook_id", Type: core.ConnectionTypeString, Label: "Webhook ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := jotform.Get(inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	region := jotform.Region(inputs)
	formID, err := forms_common.RequiredString("form_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	webhookID, err := forms_common.RequiredString("webhook_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	raw, status, err := jotform.Do(jotform.Context(flow), http.MethodDelete, "/form/"+formID+"/webhooks/"+webhookID, apiKey, region, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("JotForm request failed: %v", err)), nil
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return forms_common.ErrorResult(jotform.StatusMessage(status, raw)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted JotForm webhook %s from form %s.", webhookID, formID),
		"form_id":     formID,
		"webhook_id":  webhookID,
		"success":     true,
		"error":       "",
	}, nil
}

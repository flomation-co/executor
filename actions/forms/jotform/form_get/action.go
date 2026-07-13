// Package form_get retrieves a single JotForm form by ID.
package form_get

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
	Name         = "Get Form"
	Description  = "Retrieve a single JotForm form's details by its form ID."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+eye"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Form"},
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

	raw, status, err := jotform.Do(jotform.Context(flow), http.MethodGet, "/form/"+formID, apiKey, region, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("JotForm request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(jotform.StatusMessage(status, raw)), nil
	}

	content := jotform.Content(raw)
	title, _ := content["title"].(string)
	result := forms_common.ObjectResult(content, fmt.Sprintf("Retrieved JotForm form %q (%s).", title, formID))
	result["form_id"] = formID
	return result, nil
}

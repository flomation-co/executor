// Package form_create creates a new JotForm form.
package form_create

import (
	"encoding/json"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/jotform"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Form"
	Description  = "Create a new JotForm form from a JSON questions array and properties object."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+plus"
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
	{Name: "questions", Type: core.ConnectionTypeString, Label: "Questions (JSON)", Placeholder: `{"1":{"type":"control_textbox","text":"Name","order":"1"}}`},
	{Name: "properties", Type: core.ConnectionTypeString, Label: "Properties (JSON)", Placeholder: `{"title":"My contact form"}`},
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

	payload := map[string]interface{}{}
	if raw := forms_common.OptionalString("questions", inputs); raw != "" {
		var questions interface{}
		if err := json.Unmarshal([]byte(raw), &questions); err != nil {
			return forms_common.ErrorResult(fmt.Sprintf("Invalid questions JSON: %v", err)), nil
		}
		payload["questions"] = questions
	}
	if raw := forms_common.OptionalString("properties", inputs); raw != "" {
		var properties interface{}
		if err := json.Unmarshal([]byte(raw), &properties); err != nil {
			return forms_common.ErrorResult(fmt.Sprintf("Invalid properties JSON: %v", err)), nil
		}
		payload["properties"] = properties
	}

	body, _ := json.Marshal(payload)
	raw, status, err := jotform.Do(jotform.Context(flow), http.MethodPost, "/user/forms", apiKey, region, body)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("JotForm request failed: %v", err)), nil
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return forms_common.ErrorResult(jotform.StatusMessage(status, raw)), nil
	}

	content := jotform.Content(raw)
	formID, _ := content["id"].(string)
	result := forms_common.ObjectResult(content, fmt.Sprintf("Created JotForm form %s.", formID))
	result["form_id"] = formID
	return result, nil
}

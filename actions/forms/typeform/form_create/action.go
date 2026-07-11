// Package form_create creates a new Typeform form.
package form_create

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
	Name         = "Create Form"
	Description  = "Create a new Typeform form with a title and a JSON array of fields."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+plus"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Typeform Personal Access Token", Placeholder: "${secrets.typeform_token}", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "My contact form", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields (JSON array)", Placeholder: `[{"title":"What is your name?","type":"short_text"}]`},
	{Name: "settings", Type: core.ConnectionTypeString, Label: "Settings (JSON object)", Placeholder: `{"is_public":true}`},
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
	title, err := forms_common.RequiredString("title", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	payload := map[string]interface{}{"title": title}

	if raw := forms_common.OptionalString("fields", inputs); raw != "" {
		var fields []interface{}
		if err := json.Unmarshal([]byte(raw), &fields); err != nil {
			return forms_common.ErrorResult(fmt.Sprintf("Invalid fields JSON: %v", err)), nil
		}
		payload["fields"] = fields
	}
	if raw := forms_common.OptionalString("settings", inputs); raw != "" {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &settings); err != nil {
			return forms_common.ErrorResult(fmt.Sprintf("Invalid settings JSON: %v", err)), nil
		}
		payload["settings"] = settings
	}

	body, _ := json.Marshal(payload)
	obj, status, err := typeform.Do(typeform.Context(flow), http.MethodPost, "/forms", token, body)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Typeform request failed: %v", err)), nil
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return forms_common.ErrorResult(typeform.StatusMessage(status, obj)), nil
	}

	formID, _ := obj["id"].(string)
	formURL := displayURL(obj)
	result := forms_common.ObjectResult(obj, fmt.Sprintf("Created Typeform form %q (%s).", title, formID))
	result["form_id"] = formID
	result["form_url"] = formURL
	return result, nil
}

// displayURL extracts the public display link from a form's _links.display.
func displayURL(obj map[string]interface{}) string {
	links, ok := obj["_links"].(map[string]interface{})
	if !ok {
		return ""
	}
	url, _ := links["display"].(string)
	return url
}

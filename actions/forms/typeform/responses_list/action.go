// Package responses_list retrieves the submitted responses for a Typeform form.
package responses_list

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/typeform"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Responses"
	Description  = "Retrieve submitted responses for a Typeform form, with date and completion filters."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+envelope-open-text"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Typeform Personal Access Token", Placeholder: "${secrets.typeform_token}", Required: true},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID", Placeholder: "abc123", Required: true},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "25"},
	{Name: "since", Type: core.ConnectionTypeString, Label: "Since", Placeholder: "2026-01-01T00:00:00Z"},
	{Name: "until", Type: core.ConnectionTypeString, Label: "Until", Placeholder: "2026-12-31T23:59:59Z"},
	{Name: "completed", Type: core.ConnectionTypeString, Label: "Completed", Placeholder: "true or false", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Completed only", Value: "true"},
		{Name: "Incomplete only", Value: "false"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "responses", Type: core.ConnectionTypeObject, Label: "Responses"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_items", Type: core.ConnectionTypeInteger, Label: "Total Items"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := typeform.Get(inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	formID, err := forms_common.RequiredString("form_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	query := url.Values{}
	for _, name := range []string{"page_size", "since", "until", "completed"} {
		if v := forms_common.OptionalString(name, inputs); v != "" {
			query.Set(name, v)
		}
	}
	path := "/forms/" + formID + "/responses"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	obj, status, err := typeform.Do(typeform.Context(flow), http.MethodGet, path, token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Typeform request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(typeform.StatusMessage(status, obj)), nil
	}

	responses := make([]map[string]interface{}, 0)
	if rawItems, ok := obj["items"].([]interface{}); ok {
		for _, it := range rawItems {
			if m, ok := it.(map[string]interface{}); ok {
				responses = append(responses, m)
			}
		}
	}
	totalItems := 0
	if tc, ok := obj["total_items"].(float64); ok {
		totalItems = int(tc)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Retrieved %d response(s) for Typeform form %s (of %d total).", len(responses), formID, totalItems),
		"responses":   responses,
		"count":       len(responses),
		"total_items": totalItems,
		"success":     true,
		"error":       "",
	}, nil
}

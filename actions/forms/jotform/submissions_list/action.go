// Package submissions_list retrieves the submissions for a JotForm form.
package submissions_list

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/jotform"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Submissions"
	Description  = "Retrieve the submissions for a JotForm form, with limit, offset and filter."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+envelope-open-text"
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
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "20"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Offset", Placeholder: "0"},
	{Name: "filter", Type: core.ConnectionTypeString, Label: "Filter (JSON)", Placeholder: `{"status":"ACTIVE"}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "submissions", Type: core.ConnectionTypeObject, Label: "Submissions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	query := url.Values{}
	for _, name := range []string{"limit", "offset", "filter"} {
		if v := forms_common.OptionalString(name, inputs); v != "" {
			query.Set(name, v)
		}
	}
	path := "/form/" + formID + "/submissions"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	raw, status, err := jotform.Do(jotform.Context(flow), http.MethodGet, path, apiKey, region, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("JotForm request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(jotform.StatusMessage(status, raw)), nil
	}

	submissions := jotform.ContentList(raw)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Retrieved %d submission(s) for JotForm form %s.", len(submissions), formID),
		"submissions": submissions,
		"count":       len(submissions),
		"success":     true,
		"error":       "",
	}, nil
}

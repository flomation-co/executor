// Package submission_get retrieves a single JotForm submission by ID.
package submission_get

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
	Name         = "Get Submission"
	Description  = "Retrieve a single JotForm submission and its answers by submission ID."
	Website      = "https://www.flomation.co"
	Icon         = "envelope-open-text+eye"
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
	{Name: "submission_id", Type: core.ConnectionTypeString, Label: "Submission ID", Placeholder: "5551234567890", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "submission_id", Type: core.ConnectionTypeString, Label: "Submission ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Submission"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := jotform.Get(inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	region := jotform.Region(inputs)
	submissionID, err := forms_common.RequiredString("submission_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	raw, status, err := jotform.Do(jotform.Context(flow), http.MethodGet, "/submission/"+submissionID, apiKey, region, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("JotForm request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(jotform.StatusMessage(status, raw)), nil
	}

	content := jotform.Content(raw)
	result := forms_common.ObjectResult(content, fmt.Sprintf("Retrieved JotForm submission %s.", submissionID))
	result["submission_id"] = submissionID
	return result, nil
}

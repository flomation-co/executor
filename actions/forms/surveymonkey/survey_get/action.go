// Package survey_get retrieves a single SurveyMonkey survey by ID.
package survey_get

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
	Name         = "Get Survey"
	Description  = "Retrieve a single SurveyMonkey survey by its survey ID."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+eye"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "SurveyMonkey Access Token", Placeholder: "${secrets.surveymonkey_token}", Required: true},
	{Name: "survey_id", Type: core.ConnectionTypeString, Label: "Survey ID", Placeholder: "123456789", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "survey_id", Type: core.ConnectionTypeString, Label: "Survey ID"},
	{Name: "survey_url", Type: core.ConnectionTypeString, Label: "Survey URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Survey"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := surveymonkey.Get(inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	surveyID, err := forms_common.RequiredString("survey_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	obj, status, err := surveymonkey.Do(surveymonkey.Context(flow), http.MethodGet, "/surveys/"+surveyID, token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("SurveyMonkey request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(surveymonkey.StatusMessage(status, obj)), nil
	}

	title, _ := obj["title"].(string)
	surveyURL, _ := obj["href"].(string)
	result := forms_common.ObjectResult(obj, fmt.Sprintf("Retrieved SurveyMonkey survey %q (%s).", title, surveyID))
	result["survey_id"] = surveyID
	result["survey_url"] = surveyURL
	return result, nil
}

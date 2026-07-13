// Package response_get retrieves a single SurveyMonkey survey response by ID.
package response_get

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
	Name         = "Get Response"
	Description  = "Retrieve a single SurveyMonkey survey response by its response ID."
	Website      = "https://www.flomation.co"
	Icon         = "envelope-open-text+eye"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "SurveyMonkey Access Token", Placeholder: "${secrets.surveymonkey_token}", Required: true},
	{Name: "survey_id", Type: core.ConnectionTypeString, Label: "Survey ID", Placeholder: "123456789", Required: true},
	{Name: "response_id", Type: core.ConnectionTypeString, Label: "Response ID", Placeholder: "987654321", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "response_id", Type: core.ConnectionTypeString, Label: "Response ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Response"},
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
	responseID, err := forms_common.RequiredString("response_id", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	obj, status, err := surveymonkey.Do(surveymonkey.Context(flow), http.MethodGet, "/surveys/"+surveyID+"/responses/"+responseID, token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("SurveyMonkey request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(surveymonkey.StatusMessage(status, obj)), nil
	}

	result := forms_common.ObjectResult(obj, fmt.Sprintf("Retrieved SurveyMonkey response %s for survey %s.", responseID, surveyID))
	result["response_id"] = responseID
	return result, nil
}

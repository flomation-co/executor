// Package survey_create creates a new SurveyMonkey survey.
package survey_create

import (
	"encoding/json"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/surveymonkey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Survey"
	Description  = "Create a new SurveyMonkey survey with a title and optional JSON body override."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+plus"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "SurveyMonkey Access Token", Placeholder: "${secrets.surveymonkey_token}", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "Customer satisfaction", Required: true},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Body (JSON object)", Placeholder: `{"nickname":"CSAT 2026","language":"en"}`},
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
	title, err := forms_common.RequiredString("title", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	payload := map[string]interface{}{}
	if raw := forms_common.OptionalString("body", inputs); raw != "" {
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return forms_common.ErrorResult(fmt.Sprintf("Invalid body JSON: %v", err)), nil
		}
	}
	payload["title"] = title

	body, _ := json.Marshal(payload)
	obj, status, err := surveymonkey.Do(surveymonkey.Context(flow), http.MethodPost, "/surveys", token, body)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("SurveyMonkey request failed: %v", err)), nil
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return forms_common.ErrorResult(surveymonkey.StatusMessage(status, obj)), nil
	}

	surveyID := stringID(obj["id"])
	surveyURL, _ := obj["href"].(string)
	result := forms_common.ObjectResult(obj, fmt.Sprintf("Created SurveyMonkey survey %q (%s).", title, surveyID))
	result["survey_id"] = surveyID
	result["survey_url"] = surveyURL
	return result, nil
}

// stringID normalises an id that SurveyMonkey may return as a string.
func stringID(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Package collector_create creates a collector for a SurveyMonkey survey.
package collector_create

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
	Name         = "Create Collector"
	Description  = "Create a collector (e.g. weblink) that distributes a SurveyMonkey survey."
	Website      = "https://www.flomation.co"
	Icon         = "link+plus"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "SurveyMonkey Access Token", Placeholder: "${secrets.surveymonkey_token}", Required: true},
	{Name: "survey_id", Type: core.ConnectionTypeString, Label: "Survey ID", Placeholder: "123456789", Required: true},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "weblink", Options: []core.ConnectionOption{
		{Name: "Web Link", Value: "weblink"},
		{Name: "Email", Value: "email"},
	}},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Website link"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Body (JSON object)", Placeholder: `{"display_survey_results":false}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "collector_id", Type: core.ConnectionTypeString, Label: "Collector ID"},
	{Name: "collector_url", Type: core.ConnectionTypeString, Label: "Collector URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Collector"},
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

	payload := map[string]interface{}{}
	if raw := forms_common.OptionalString("body", inputs); raw != "" {
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return forms_common.ErrorResult(fmt.Sprintf("Invalid body JSON: %v", err)), nil
		}
	}
	collectorType := forms_common.OptionalString("type", inputs)
	if collectorType == "" {
		collectorType = "weblink"
	}
	payload["type"] = collectorType
	if name := forms_common.OptionalString("name", inputs); name != "" {
		payload["name"] = name
	}

	body, _ := json.Marshal(payload)
	obj, status, err := surveymonkey.Do(surveymonkey.Context(flow), http.MethodPost, "/surveys/"+surveyID+"/collectors", token, body)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("SurveyMonkey request failed: %v", err)), nil
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return forms_common.ErrorResult(surveymonkey.StatusMessage(status, obj)), nil
	}

	collectorID := ""
	if s, ok := obj["id"].(string); ok {
		collectorID = s
	}
	collectorURL, _ := obj["url"].(string)
	result := forms_common.ObjectResult(obj, fmt.Sprintf("Created %s collector %s for SurveyMonkey survey %s.", collectorType, collectorID, surveyID))
	result["collector_id"] = collectorID
	result["collector_url"] = collectorURL
	return result, nil
}

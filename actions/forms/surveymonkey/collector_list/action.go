// Package collector_list lists the collectors attached to a SurveyMonkey survey.
package collector_list

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
	Name         = "List Collectors"
	Description  = "List the collectors (distribution channels) attached to a SurveyMonkey survey."
	Website      = "https://www.flomation.co"
	Icon         = "link+list"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "SurveyMonkey Access Token", Placeholder: "${secrets.surveymonkey_token}", Required: true},
	{Name: "survey_id", Type: core.ConnectionTypeString, Label: "Survey ID", Placeholder: "123456789", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Collectors"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
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

	obj, status, err := surveymonkey.Do(surveymonkey.Context(flow), http.MethodGet, "/surveys/"+surveyID+"/collectors", token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("SurveyMonkey request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(surveymonkey.StatusMessage(status, obj)), nil
	}

	items := make([]map[string]interface{}, 0)
	if rawItems, ok := obj["data"].([]interface{}); ok {
		for _, it := range rawItems {
			if m, ok := it.(map[string]interface{}); ok {
				items = append(items, m)
			}
		}
	}
	total := 0
	if tc, ok := obj["total"].(float64); ok {
		total = int(tc)
	}

	result := forms_common.ListResult(items, fmt.Sprintf("Found %d collector(s) for SurveyMonkey survey %s (of %d total).", len(items), surveyID, total))
	result["total"] = total
	return result, nil
}

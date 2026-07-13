// Package responses_list retrieves the bulk responses for a SurveyMonkey survey.
package responses_list

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/surveymonkey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Responses"
	Description  = "Retrieve bulk responses for a SurveyMonkey survey, with page and per-page paging."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+envelope-open-text"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "SurveyMonkey Access Token", Placeholder: "${secrets.surveymonkey_token}", Required: true},
	{Name: "survey_id", Type: core.ConnectionTypeString, Label: "Survey ID", Placeholder: "123456789", Required: true},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1"},
	{Name: "per_page", Type: core.ConnectionTypeInteger, Label: "Per Page", Placeholder: "50"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "responses", Type: core.ConnectionTypeObject, Label: "Responses"},
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

	query := url.Values{}
	for _, name := range []string{"page", "per_page"} {
		if v := forms_common.OptionalString(name, inputs); v != "" {
			query.Set(name, v)
		}
	}
	path := "/surveys/" + surveyID + "/responses/bulk"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	obj, status, err := surveymonkey.Do(surveymonkey.Context(flow), http.MethodGet, path, token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("SurveyMonkey request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(surveymonkey.StatusMessage(status, obj)), nil
	}

	responses := make([]map[string]interface{}, 0)
	if rawItems, ok := obj["data"].([]interface{}); ok {
		for _, it := range rawItems {
			if m, ok := it.(map[string]interface{}); ok {
				responses = append(responses, m)
			}
		}
	}
	total := 0
	if tc, ok := obj["total"].(float64); ok {
		total = int(tc)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Retrieved %d response(s) for SurveyMonkey survey %s (of %d total).", len(responses), surveyID, total),
		"responses":   responses,
		"count":       len(responses),
		"total":       total,
		"success":     true,
		"error":       "",
	}, nil
}

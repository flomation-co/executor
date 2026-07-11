// Package webhook_create registers a SurveyMonkey webhook.
package webhook_create

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/surveymonkey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Webhook"
	Description  = "Register a SurveyMonkey webhook that fires on survey or response events."
	Website      = "https://www.flomation.co"
	Icon         = "webhook+plus"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "SurveyMonkey Access Token", Placeholder: "${secrets.surveymonkey_token}", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Flomation responses", Required: true},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type", Placeholder: "response_completed", Options: []core.ConnectionOption{
		{Name: "Response completed", Value: "response_completed"},
		{Name: "Response created", Value: "response_created"},
		{Name: "Response updated", Value: "response_updated"},
		{Name: "Response disqualified", Value: "response_disqualified"},
		{Name: "Response overquota", Value: "response_overquota"},
		{Name: "Survey created", Value: "survey_created"},
		{Name: "Survey updated", Value: "survey_updated"},
		{Name: "Survey deleted", Value: "survey_deleted"},
		{Name: "Collector created", Value: "collector_created"},
		{Name: "Collector updated", Value: "collector_updated"},
		{Name: "Collector deleted", Value: "collector_deleted"},
	}},
	{Name: "object_type", Type: core.ConnectionTypeString, Label: "Object Type", Placeholder: "survey", Options: []core.ConnectionOption{
		{Name: "Survey", Value: "survey"},
		{Name: "Collector", Value: "collector"},
	}},
	{Name: "object_ids", Type: core.ConnectionTypeString, Label: "Object IDs (comma-separated)", Placeholder: "123456789,987654321"},
	{Name: "subscription_url", Type: core.ConnectionTypeString, Label: "Subscription URL", Placeholder: "https://launch.flomation.app/webhook/…", Required: true},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Body (JSON object)", Placeholder: `{"object_ids":["123"]}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "webhook_id", Type: core.ConnectionTypeString, Label: "Webhook ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Webhook"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := surveymonkey.Get(inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	name, err := forms_common.RequiredString("name", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	subscriptionURL, err := forms_common.RequiredString("subscription_url", inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	payload := map[string]interface{}{}
	if raw := forms_common.OptionalString("body", inputs); raw != "" {
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return forms_common.ErrorResult(fmt.Sprintf("Invalid body JSON: %v", err)), nil
		}
	}
	payload["name"] = name
	payload["subscription_url"] = subscriptionURL
	if v := forms_common.OptionalString("event_type", inputs); v != "" {
		payload["event_type"] = v
	}
	if v := forms_common.OptionalString("object_type", inputs); v != "" {
		payload["object_type"] = v
	}
	if v := forms_common.OptionalString("object_ids", inputs); v != "" {
		ids := make([]string, 0)
		for _, part := range strings.Split(v, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				ids = append(ids, trimmed)
			}
		}
		if len(ids) > 0 {
			payload["object_ids"] = ids
		}
	}

	body, _ := json.Marshal(payload)
	obj, status, err := surveymonkey.Do(surveymonkey.Context(flow), http.MethodPost, "/webhooks", token, body)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("SurveyMonkey request failed: %v", err)), nil
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return forms_common.ErrorResult(surveymonkey.StatusMessage(status, obj)), nil
	}

	webhookID := ""
	if s, ok := obj["id"].(string); ok {
		webhookID = s
	}
	result := forms_common.ObjectResult(obj, fmt.Sprintf("Registered SurveyMonkey webhook %q (%s) → %s.", name, webhookID, subscriptionURL))
	result["webhook_id"] = webhookID
	return result, nil
}

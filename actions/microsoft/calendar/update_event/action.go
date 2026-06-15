// Package update_event updates an existing calendar event in Microsoft Outlook.
package update_event

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Update Event"
	Description  = "Update an existing calendar event in Microsoft Outlook"
	Website      = "https://www.flomation.co"
	Icon         = "calendar+pencil"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID", Required: true},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject"},
	{Name: "start_time", Type: core.ConnectionTypeString, Label: "Start Time"},
	{Name: "end_time", Type: core.ConnectionTypeString, Label: "End Time"},
	{Name: "timezone", Type: core.ConnectionTypeString, Label: "Timezone"},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Body"},
	{Name: "location", Type: core.ConnectionTypeString, Label: "Location"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_CALENDAR}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	eventID := microsoft.OptStr("event_id", inputs)
	subject := microsoft.OptStr("subject", inputs)
	startTime := microsoft.OptStr("start_time", inputs)
	endTime := microsoft.OptStr("end_time", inputs)
	timezone := microsoft.OptStr("timezone", inputs)
	bodyContent := microsoft.OptStr("body", inputs)
	location := microsoft.OptStr("location", inputs)
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	if eventID == "" {
		return microsoft.ErrorResult("event_id is required")
	}

	tokens, err := microsoft.FetchTokens(flow, credential, "calendar")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	eventBody := make(map[string]interface{})

	if subject != "" {
		eventBody["subject"] = subject
	}
	if startTime != "" {
		tz := timezone
		if tz == "" {
			tz = "UTC"
		}
		eventBody["start"] = map[string]string{
			"dateTime": startTime,
			"timeZone": tz,
		}
	}
	if endTime != "" {
		tz := timezone
		if tz == "" {
			tz = "UTC"
		}
		eventBody["end"] = map[string]string{
			"dateTime": endTime,
			"timeZone": tz,
		}
	}
	if bodyContent != "" {
		eventBody["body"] = map[string]string{
			"contentType": "HTML",
			"content":     bodyContent,
		}
	}
	if location != "" {
		eventBody["location"] = map[string]string{
			"displayName": location,
		}
	}

	if len(eventBody) == 0 {
		return microsoft.ErrorResult("no fields provided to update")
	}

	reqBody, err := json.Marshal(eventBody)
	if err != nil {
		return microsoft.ErrorResult("failed to build request: " + err.Error())
	}

	endpoint := fmt.Sprintf("%s/me/events/%s", microsoft.GraphAPI, eventID)
	status, body, err := microsoft.DoRequest(flow, "PATCH", endpoint, token.AccessToken, reqBody)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(body)))
	}

	toolResult := fmt.Sprintf("Updated event %s", eventID)
	if subject != "" {
		toolResult = fmt.Sprintf("Updated event '%s'", subject)
	}

	return map[string]interface{}{
		"tool_result": toolResult,
		"success":     true,
		"error":       "",
	}, nil
}

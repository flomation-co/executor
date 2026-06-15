// Package create_event creates a new calendar event in Microsoft Outlook.
package create_event

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Event"
	Description  = "Create a new calendar event in Microsoft Outlook"
	Website      = "https://www.flomation.co"
	Icon         = "calendar+plus"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Required: true},
	{Name: "start_time", Type: core.ConnectionTypeString, Label: "Start Time", Required: true, Placeholder: "2026-06-15T09:00:00"},
	{Name: "end_time", Type: core.ConnectionTypeString, Label: "End Time", Required: true, Placeholder: "2026-06-15T10:00:00"},
	{Name: "timezone", Type: core.ConnectionTypeString, Label: "Timezone", Placeholder: "UTC"},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Body"},
	{Name: "location", Type: core.ConnectionTypeString, Label: "Location"},
	{Name: "attendees", Type: core.ConnectionTypeString, Label: "Attendees", Placeholder: "user@example.com, user2@example.com"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_CALENDAR}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	subject := microsoft.OptStr("subject", inputs)
	startTime := microsoft.OptStr("start_time", inputs)
	endTime := microsoft.OptStr("end_time", inputs)
	timezone := microsoft.OptStr("timezone", inputs)
	bodyContent := microsoft.OptStr("body", inputs)
	location := microsoft.OptStr("location", inputs)
	attendeesStr := microsoft.OptStr("attendees", inputs)
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	if subject == "" {
		return microsoft.ErrorResult("subject is required")
	}
	if startTime == "" {
		return microsoft.ErrorResult("start_time is required")
	}
	if endTime == "" {
		return microsoft.ErrorResult("end_time is required")
	}
	if timezone == "" {
		timezone = "UTC"
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

	eventBody := map[string]interface{}{
		"subject": subject,
		"start": map[string]string{
			"dateTime": startTime,
			"timeZone": timezone,
		},
		"end": map[string]string{
			"dateTime": endTime,
			"timeZone": timezone,
		},
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

	if attendeesStr != "" {
		parts := strings.Split(attendeesStr, ",")
		var attendees []map[string]interface{}
		for _, p := range parts {
			email := strings.TrimSpace(p)
			if email == "" {
				continue
			}
			attendees = append(attendees, map[string]interface{}{
				"emailAddress": map[string]string{
					"address": email,
					"name":    email,
				},
				"type": "required",
			})
		}
		if len(attendees) > 0 {
			eventBody["attendees"] = attendees
		}
	}

	reqBody, err := json.Marshal(eventBody)
	if err != nil {
		return microsoft.ErrorResult("failed to build request: " + err.Error())
	}

	endpoint := fmt.Sprintf("%s/me/events", microsoft.GraphAPI)
	status, body, err := microsoft.DoRequest(flow, "POST", endpoint, token.AccessToken, reqBody)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(body)))
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return microsoft.ErrorResult("failed to parse response: " + err.Error())
	}

	toolResult := fmt.Sprintf("Created event '%s' from %s to %s", subject, startTime, endTime)
	if location != "" {
		toolResult += " at " + location
	}

	return map[string]interface{}{
		"tool_result": toolResult,
		"event_id":    resp.ID,
		"success":     true,
		"error":       "",
	}, nil
}

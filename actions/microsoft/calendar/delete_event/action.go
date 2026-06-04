// Package delete_event deletes a calendar event from Microsoft Outlook.
package delete_event

import (
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Delete Event"
	Description  = "Delete a calendar event from Microsoft Outlook"
	Website      = "https://www.flomation.co"
	Icon         = "calendar+trash"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_CALENDAR}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	eventID := microsoft.OptStr("event_id", inputs)
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

	endpoint := fmt.Sprintf("%s/me/events/%s", microsoft.GraphAPI, eventID)
	status, body, err := microsoft.DoRequest(flow, "DELETE", endpoint, token.AccessToken, nil)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status != 204 && (status < 200 || status >= 300) {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(body)))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted event %s", eventID),
		"success":     true,
		"error":       "",
	}, nil
}

// Package get_event retrieves a single calendar event from Microsoft Outlook.
package get_event

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Event"
	Description  = "Retrieve details of a specific calendar event"
	Website      = "https://www.flomation.co"
	Icon         = "calendar+eye"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction

	selectFields = "id,subject,body,start,end,location,organizer,attendees,isAllDay,recurrence"
)

var Inputs = [...]core.Connection{
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_CALENDAR}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject"},
	{Name: "start", Type: core.ConnectionTypeString, Label: "Start Time"},
	{Name: "end_time", Type: core.ConnectionTypeString, Label: "End Time"},
	{Name: "location", Type: core.ConnectionTypeString, Label: "Location"},
	{Name: "organizer", Type: core.ConnectionTypeString, Label: "Organiser"},
	{Name: "attendees", Type: core.ConnectionTypeString, Label: "Attendees"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Body"},
	{Name: "is_all_day", Type: core.ConnectionTypeBoolean, Label: "All Day Event"},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event (JSON)"},
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

	endpoint := fmt.Sprintf("%s/me/events/%s?$select=%s",
		microsoft.GraphAPI, eventID, selectFields)

	status, body, err := microsoft.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(body)))
	}

	var event struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
		Body    struct {
			Content string `json:"content"`
		} `json:"body"`
		Start struct {
			DateTime string `json:"dateTime"`
		} `json:"start"`
		End struct {
			DateTime string `json:"dateTime"`
		} `json:"end"`
		Location struct {
			DisplayName string `json:"displayName"`
		} `json:"location"`
		Organizer struct {
			EmailAddress struct {
				Address string `json:"address"`
			} `json:"emailAddress"`
		} `json:"organizer"`
		Attendees []struct {
			EmailAddress struct {
				Address string `json:"address"`
				Name    string `json:"name"`
			} `json:"emailAddress"`
		} `json:"attendees"`
		IsAllDay bool `json:"isAllDay"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return microsoft.ErrorResult("failed to parse response: " + err.Error())
	}

	var attendeeList []string
	for _, a := range event.Attendees {
		if a.EmailAddress.Name != "" {
			attendeeList = append(attendeeList, fmt.Sprintf("%s <%s>", a.EmailAddress.Name, a.EmailAddress.Address))
		} else {
			attendeeList = append(attendeeList, a.EmailAddress.Address)
		}
	}
	attendeesJSON, _ := json.Marshal(attendeeList)

	toolResult := fmt.Sprintf("Event: %s\nWhen: %s to %s\nLocation: %s\nOrganiser: %s",
		event.Subject, event.Start.DateTime, event.End.DateTime,
		event.Location.DisplayName, event.Organizer.EmailAddress.Address)

	return map[string]interface{}{
		"tool_result": toolResult,
		"subject":     event.Subject,
		"start":       event.Start.DateTime,
		"end_time":    event.End.DateTime,
		"location":    event.Location.DisplayName,
		"organizer":   event.Organizer.EmailAddress.Address,
		"attendees":   string(attendeesJSON),
		"body":        event.Body.Content,
		"is_all_day":  event.IsAllDay,
		"event":       string(body),
		"success":     true,
		"error":       "",
	}, nil
}

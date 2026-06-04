// Package list_events lists calendar events from a Microsoft Outlook calendar.
package list_events

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
	Name         = "List Events"
	Description  = "List calendar events from a Microsoft Outlook calendar"
	Website      = "https://www.flomation.co"
	Icon         = "calendar+list"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction

	selectFields = "id,subject,start,end,location,organizer,isAllDay,bodyPreview"
)

var Inputs = [...]core.Connection{
	{Name: "start_date", Type: core.ConnectionTypeString, Label: "Start Date", Placeholder: "2026-06-01T00:00:00"},
	{Name: "end_date", Type: core.ConnectionTypeString, Label: "End Date", Placeholder: "2026-06-30T23:59:59"},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_CALENDAR}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "events", Type: core.ConnectionTypeString, Label: "Events (JSON)"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Event Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	startDate := microsoft.OptStr("start_date", inputs)
	endDate := microsoft.OptStr("end_date", inputs)
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)
	maxResults := microsoft.OptInt("max_results", inputs, 25)

	tokens, err := microsoft.FetchTokens(flow, credential, "calendar")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	endpoint := fmt.Sprintf("%s/me/events?$top=%d&$orderby=start/dateTime&$select=%s",
		microsoft.GraphAPI, maxResults, selectFields)

	if startDate != "" {
		filter := fmt.Sprintf("start/dateTime ge '%s'", startDate)
		if endDate != "" {
			filter += fmt.Sprintf(" and end/dateTime le '%s'", endDate)
		}
		endpoint += "&$filter=" + filter
	}

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

	var resp struct {
		Value []struct {
			ID          string `json:"id"`
			Subject     string `json:"subject"`
			Start       struct {
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
			IsAllDay    bool   `json:"isAllDay"`
			BodyPreview string `json:"bodyPreview"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return microsoft.ErrorResult("failed to parse response: " + err.Error())
	}

	eventsJSON, _ := json.Marshal(resp.Value)
	count := len(resp.Value)

	var summaries []string
	for _, e := range resp.Value {
		summary := fmt.Sprintf("- %s (%s to %s)", e.Subject, e.Start.DateTime, e.End.DateTime)
		if e.Location.DisplayName != "" {
			summary += " at " + e.Location.DisplayName
		}
		summaries = append(summaries, summary)
	}

	toolResult := fmt.Sprintf("Found %d calendar event(s):\n%s", count, strings.Join(summaries, "\n"))
	if count == 0 {
		toolResult = "No calendar events found"
	}

	return map[string]interface{}{
		"tool_result": toolResult,
		"events":      string(eventsJSON),
		"count":       fmt.Sprintf("%d", count),
		"success":     true,
		"error":       "",
	}, nil
}

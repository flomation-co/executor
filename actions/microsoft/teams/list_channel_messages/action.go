// Package list_channel_messages retrieves recent messages from a Microsoft Teams channel.
package list_channel_messages

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Channel Messages"
	Description  = "Retrieve recent messages from a Microsoft Teams channel"
	Website      = "https://www.flomation.co"
	Icon         = "microsoft+eye"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "team_id", Type: core.ConnectionTypeString, Label: "Team ID", Required: true},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID", Required: true},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_TEAMS}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "messages", Type: core.ConnectionTypeString, Label: "Messages (JSON array)"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Message Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	teamID := microsoft.OptStr("team_id", inputs)
	if teamID == "" {
		return microsoft.ErrorResult("team_id is required")
	}
	channelID := microsoft.OptStr("channel_id", inputs)
	if channelID == "" {
		return microsoft.ErrorResult("channel_id is required")
	}

	maxResults := microsoft.OptInt("max_results", inputs, 20)
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	tokens, err := microsoft.FetchTokens(flow, credential, "teams")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	endpoint := fmt.Sprintf("%s/teams/%s/channels/%s/messages?$top=%d", microsoft.GraphAPI, teamID, channelID, maxResults)

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
		Value []map[string]interface{} `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse response: %s", err.Error()))
	}

	messagesJSON, _ := json.Marshal(resp.Value)

	// Build tool_result summary with message previews
	summary := fmt.Sprintf("Retrieved %d message(s)", len(resp.Value))
	for i, msg := range resp.Value {
		if i >= 5 {
			summary += fmt.Sprintf("\n... and %d more", len(resp.Value)-5)
			break
		}
		sender := "Unknown"
		if from, ok := msg["from"].(map[string]interface{}); ok {
			if user, ok := from["user"].(map[string]interface{}); ok {
				if name, ok := user["displayName"].(string); ok {
					sender = name
				}
			}
		}
		preview := ""
		if bodyObj, ok := msg["body"].(map[string]interface{}); ok {
			if content, ok := bodyObj["content"].(string); ok {
				preview = content
				if len(preview) > 50 {
					preview = preview[:50] + "..."
				}
			}
		}
		summary += fmt.Sprintf("\n- %s: %s", sender, preview)
	}

	return map[string]interface{}{
		"tool_result": summary,
		"messages":    string(messagesJSON),
		"count":       fmt.Sprintf("%d", len(resp.Value)),
		"success":     true,
		"error":       "",
	}, nil
}

// Package create_channel creates a new channel in a Microsoft Teams team.
package create_channel

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Channel"
	Description  = "Create a new channel in a Microsoft Teams team"
	Website      = "https://www.flomation.co"
	Icon         = "microsoft+plus"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "team_id", Type: core.ConnectionTypeString, Label: "Team ID", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Channel Name", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Channel Description"},
	{Name: "membership_type", Type: core.ConnectionTypeString, Label: "Membership Type", Options: []core.ConnectionOption{
		{Name: "Standard", Value: "standard"},
		{Name: "Private", Value: "private"},
		{Name: "Shared", Value: "shared"},
	}},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_TEAMS}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	teamID := microsoft.OptStr("team_id", inputs)
	if teamID == "" {
		return microsoft.ErrorResult("team_id is required")
	}
	name := microsoft.OptStr("name", inputs)
	if name == "" {
		return microsoft.ErrorResult("name is required")
	}

	description := microsoft.OptStr("description", inputs)
	membershipType := microsoft.OptStr("membership_type", inputs)
	if membershipType == "" {
		membershipType = "standard"
	}
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

	payload := map[string]interface{}{
		"displayName":    name,
		"membershipType": membershipType,
	}
	if description != "" {
		payload["description"] = description
	}
	reqBody, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("%s/teams/%s/channels", microsoft.GraphAPI, teamID)

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

	var resp map[string]interface{}
	_ = json.Unmarshal(body, &resp)

	channelID := ""
	if id, ok := resp["id"].(string); ok {
		channelID = id
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Channel '%s' created successfully", name),
		"channel_id":  channelID,
		"success":     true,
		"error":       "",
	}, nil
}

package slack

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Slack Trigger"
	Description  = "Triggers a flow when a Slack message or app mention is received"
	Website      = "https://www.flomation.co"
	Icon         = "hashtag"
	Date         = "03/04/2026"
	Type         = core.ActionTypeTrigger
)

var Outputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name"},
	{Name: "sender", Type: core.ConnectionTypeString, Label: "Sender"},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Message Text"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Message Timestamp"},
	{Name: "thread_ts", Type: core.ConnectionTypeString, Label: "Thread Timestamp"},
	{Name: "team_id", Type: core.ConnectionTypeString, Label: "Team ID"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID"},
	{Name: "channel_type", Type: core.ConnectionTypeString, Label: "Channel Type"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing slack trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	// Map 'content' to convenience aliases if not already set
	if _, ok := result["content"]; !ok {
		if text, ok := result["text"]; ok {
			result["content"] = text
		}
	}

	return result, nil
}

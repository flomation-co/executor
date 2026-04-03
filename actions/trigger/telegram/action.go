package telegram

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Telegram Trigger"
	Description  = "Triggers a flow when a Telegram message is received"
	Website      = "https://www.flomation.co"
	Icon         = "paper-plane"
	Date         = "03/04/2026"
	Type         = core.ActionTypeTrigger
)

var Outputs = [...]core.Connection{
	{Name: "chat_id", Type: core.ConnectionTypeString, Label: "Chat ID"},
	{Name: "chat_type", Type: core.ConnectionTypeString, Label: "Chat Type"},
	{Name: "sender", Type: core.ConnectionTypeString, Label: "Sender"},
	{Name: "sender_id", Type: core.ConnectionTypeString, Label: "Sender ID"},
	{Name: "sender_username", Type: core.ConnectionTypeString, Label: "Sender Username"},
	{Name: "message_text", Type: core.ConnectionTypeString, Label: "Message Text"},
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Content"},
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing telegram trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	// Map 'content' to 'message_text' for convenience if not already set
	if _, ok := result["message_text"]; !ok {
		if content, ok := result["content"]; ok {
			result["message_text"] = content
		}
	}

	return result, nil
}

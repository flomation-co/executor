// Package twilio_sms triggers a flow when an SMS is received via Twilio.
package twilio_sms

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Twilio SMS Trigger"
	Description  = "Triggers a flow when an SMS message is received via Twilio"
	Website      = "https://www.flomation.co"
	Icon         = "comment-sms"
	Date         = "29/05/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "account_sid", Type: core.ConnectionTypeString, Label: "Twilio Account SID", Placeholder: "${secrets.twilio_account_sid}", Required: true},
	{Name: "auth_token", Type: core.ConnectionTypeString, Label: "Twilio Auth Token", Placeholder: "${secrets.twilio_auth_token}", Required: true},
	{Name: "phone_number", Type: core.ConnectionTypeString, Label: "Twilio Phone Number", Placeholder: "+15551234567", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "from", Type: core.ConnectionTypeString, Label: "From Number"},
	{Name: "to", Type: core.ConnectionTypeString, Label: "To Number"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Message Content"},
	{Name: "message_text", Type: core.ConnectionTypeString, Label: "Message Text"},
	{Name: "message_sid", Type: core.ConnectionTypeString, Label: "Message SID"},
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID"},
	{Name: "channel_type", Type: core.ConnectionTypeString, Label: "Channel Type"},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("executing twilio SMS trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	// Ensure content and message_text are both populated
	if _, ok := result["message_text"]; !ok {
		if content, ok := result["content"]; ok {
			result["message_text"] = content
		}
	}
	if _, ok := result["content"]; !ok {
		if text, ok := result["message_text"]; ok {
			result["content"] = text
		}
	}

	return result, nil
}

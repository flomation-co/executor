// Package twilio_voice triggers a flow when an incoming voice call is received via Twilio.
package twilio_voice

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Twilio Voice Trigger"
	Description  = "Triggers a flow when a voice call is received via Twilio"
	Website      = "https://www.flomation.co"
	Icon         = "phone"
	Date         = "29/05/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "account_sid", Type: core.ConnectionTypeString, Label: "Twilio Account SID", Placeholder: "${secrets.twilio_account_sid}", Required: true},
	{Name: "auth_token", Type: core.ConnectionTypeString, Label: "Twilio Auth Token", Placeholder: "${secrets.twilio_auth_token}", Required: true},
	{Name: "phone_number", Type: core.ConnectionTypeString, Label: "Twilio Phone Number", Placeholder: "+15551234567", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "from", Type: core.ConnectionTypeString, Label: "Caller Number"},
	{Name: "to", Type: core.ConnectionTypeString, Label: "Called Number"},
	{Name: "call_sid", Type: core.ConnectionTypeString, Label: "Call SID"},
	{Name: "stream_sid", Type: core.ConnectionTypeString, Label: "Stream SID"},
	{Name: "session_id", Type: core.ConnectionTypeString, Label: "Voice Session ID"},
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID"},
	{Name: "channel_type", Type: core.ConnectionTypeString, Label: "Channel Type"},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID"},
	{Name: "is_voice", Type: core.ConnectionTypeBoolean, Label: "Is Voice Call"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("executing twilio voice trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	// Ensure is_voice is always set
	result["is_voice"] = true

	return result, nil
}

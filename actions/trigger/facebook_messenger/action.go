package facebook_messenger

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Facebook Messenger Trigger"
	Description  = "Triggers a flow when a message is received via Facebook Messenger"
	Website      = "https://www.flomation.co"
	Icon         = "facebook-messenger"
	Date         = "24/05/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Facebook Page ID", Placeholder: "Your Facebook Page ID", Required: true},
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "Facebook User Token", Placeholder: "${credentials.facebook}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeString, Label: "App Secret", Placeholder: "${secrets.facebook_app_secret}"},
}

var Outputs = [...]core.Connection{
	{Name: "sender_id", Type: core.ConnectionTypeString, Label: "Sender PSID"},
	{Name: "message_text", Type: core.ConnectionTypeString, Label: "Message Text"},
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID"},
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Page ID"},
	{Name: "page_access_token", Type: core.ConnectionTypeString, Label: "Page Access Token"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Timestamp"},
	{Name: "has_attachments", Type: core.ConnectionTypeBoolean, Label: "Has Attachments"},
	{Name: "is_postback", Type: core.ConnectionTypeBoolean, Label: "Is Postback"},
	{Name: "postback_title", Type: core.ConnectionTypeString, Label: "Postback Title"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Content"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Facebook Messenger trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	if _, ok := result["message_text"]; !ok {
		if content, ok := result["content"]; ok {
			result["message_text"] = content
		}
	}

	return result, nil
}

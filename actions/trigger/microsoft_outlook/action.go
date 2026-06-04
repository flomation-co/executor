// Package microsoft_outlook is a trigger action that fires when a new email
// arrives in a connected Microsoft 365 account. The mail polling service in
// Launch monitors for new messages via the Microsoft Graph API.
package microsoft_outlook

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Microsoft Outlook Trigger"
	Description  = "Triggers a flow when a new email arrives in a connected Microsoft 365 account"
	Website      = "https://www.flomation.co"
	Icon         = "microsoft+envelope"
	Date         = "04/06/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "folder_id", Type: core.ConnectionTypeString, Label: "Folder ID", Placeholder: "Inbox (leave empty for Inbox)"},
	{Name: "filter", Type: core.ConnectionTypeString, Label: "OData Filter", Placeholder: "isRead eq false"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account Filter (email or label)"},
}

var Outputs = [...]core.Connection{
	{Name: "email_id", Type: core.ConnectionTypeString, Label: "Email ID"},
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID"},
	{Name: "from", Type: core.ConnectionTypeString, Label: "From"},
	{Name: "to", Type: core.ConnectionTypeString, Label: "To"},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject"},
	{Name: "body_preview", Type: core.ConnectionTypeString, Label: "Body Preview"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Body"},
	{Name: "received_at", Type: core.ConnectionTypeString, Label: "Received At"},
	{Name: "has_attachments", Type: core.ConnectionTypeBoolean, Label: "Has Attachments"},
	{Name: "importance", Type: core.ConnectionTypeString, Label: "Importance"},
	{Name: "is_read", Type: core.ConnectionTypeBoolean, Label: "Is Read"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Microsoft Outlook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}

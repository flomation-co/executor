// Package teams is a trigger action that fires when a message is received
// in a Microsoft Teams conversation via the Bot Framework webhook.
package teams

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Teams Trigger"
	Description  = "Triggers a flow when a message is received in Microsoft Teams"
	Website      = "https://www.flomation.co"
	Icon         = "microsoft+comments"
	Date         = "05/06/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "app_id", Type: core.ConnectionTypeString, Label: "App ID", Placeholder: "${secrets.teams_app_id}", Required: true},
	{Name: "app_password", Type: core.ConnectionTypeSecret, Label: "App Password", Placeholder: "${secrets.teams_app_password}", Required: true},
	{Name: "tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "${secrets.teams_tenant_id} (optional, single-tenant only)"},
}

var Outputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID (AAD Object ID)"},
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name"},
	{Name: "sender", Type: core.ConnectionTypeString, Label: "Sender"},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Conversation ID"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Message Text"},
	{Name: "conversation_type", Type: core.ConnectionTypeString, Label: "Conversation Type"},
	{Name: "teams_channel_id", Type: core.ConnectionTypeString, Label: "Teams Channel ID"},
	{Name: "teams_team_id", Type: core.ConnectionTypeString, Label: "Teams Team ID"},
	{Name: "activity_id", Type: core.ConnectionTypeString, Label: "Activity ID"},
	{Name: "service_url", Type: core.ConnectionTypeString, Label: "Service URL"},
	{Name: "tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID"},
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID"},
	{Name: "channel_type", Type: core.ConnectionTypeString, Label: "Channel Type"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Teams trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}

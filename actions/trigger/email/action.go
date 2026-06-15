// Package email is a trigger action that fires when a new email arrives
// in a connected Gmail account. The email polling service in Launch
// monitors for new messages and sends the trigger data to the flow.
package email

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Email Trigger"
	Description  = "Triggers a flow when a new email arrives in a connected Gmail account"
	Website      = "https://www.flomation.co"
	Icon         = "envelope"
	Date         = "08/04/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_GMAIL}", Required: true},
	{Name: "gmail_query", Type: core.ConnectionTypeString, Label: "Gmail Search Filter", Placeholder: "is:unread"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account Filter (email or label)"},
}

var Outputs = [...]core.Connection{
	{Name: "email_id", Type: core.ConnectionTypeString, Label: "Email ID"},
	{Name: "thread_id", Type: core.ConnectionTypeString, Label: "Thread ID"},
	{Name: "from", Type: core.ConnectionTypeString, Label: "From"},
	{Name: "to", Type: core.ConnectionTypeString, Label: "To"},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject"},
	{Name: "snippet", Type: core.ConnectionTypeString, Label: "Snippet"},
	{Name: "body_text", Type: core.ConnectionTypeString, Label: "Body (text)"},
	{Name: "date", Type: core.ConnectionTypeString, Label: "Date"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels"},
	{Name: "has_attachments", Type: core.ConnectionTypeBoolean, Label: "Has Attachments"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing email trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}

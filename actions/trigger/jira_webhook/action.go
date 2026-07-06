package jira_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Jira Webhook Trigger"
	Description  = "Triggers a flow when a Jira issue or comment event occurs (created, updated or deleted). The webhook is registered automatically; optionally filter with JQL, and set a signing secret to have Jira's HMAC-SHA256 payload signature verified."
	Website      = "https://www.flomation.co"
	Icon         = "jira"
	Date         = "06/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "events", Type: core.ConnectionTypeMultiSelect, Label: "Events", Required: true, Options: []core.ConnectionOption{
		{Name: "Issue Created", Value: "jira:issue_created"},
		{Name: "Issue Updated", Value: "jira:issue_updated"},
		{Name: "Issue Deleted", Value: "jira:issue_deleted"},
		{Name: "Comment Created", Value: "comment_created"},
		{Name: "Comment Updated", Value: "comment_updated"},
		{Name: "Comment Deleted", Value: "comment_deleted"},
	}},
	{Name: "jql", Type: core.ConnectionTypeString, Label: "JQL Filter", Placeholder: "Optional — only fire for issues matching this JQL, e.g. project = SCRUM AND priority = High"},
	{Name: "secret", Type: core.ConnectionTypeSecret, Label: "Signing Secret", Placeholder: "Optional — HMAC-SHA256 signing secret (less than 128 chars)"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "webhook_event", Type: core.ConnectionTypeString, Label: "Event"},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key"},
	{Name: "issue_id", Type: core.ConnectionTypeString, Label: "Issue ID"},
	{Name: "user", Type: core.ConnectionTypeString, Label: "User"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Timestamp"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they carry the credentials or internal registration settings. The
// launch service injects the event fields (webhook_event, issue_key, …) at fire
// time, and Execute echoes those through to the outputs.
var configInputs = map[string]bool{
	"url":       true,
	"email":     true,
	"api_token": true,
	"events":    true,
	"jql":       true,
	"secret":    true,
	"__node_id": true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Jira webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	result["content"] = buildContentSummary(result)

	return result, nil
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func buildContentSummary(data map[string]interface{}) string {
	event := str(data["webhook_event"])
	key := str(data["issue_key"])
	if event == "" {
		event = "event"
	}
	if key != "" {
		return fmt.Sprintf("[Jira] %s on %s", event, key)
	}
	return fmt.Sprintf("[Jira] %s", event)
}

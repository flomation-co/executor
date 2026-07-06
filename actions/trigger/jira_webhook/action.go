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
	// Option Names deliberately repeat across groups ("Created" appears under
	// Issue, Comment, Worklog, …) — the Group prefixes them in the UI, and Value
	// (the canonical Jira event id) is unique, so this is unambiguous. The
	// "Configuration" group holds Jira's GLOBAL toggle events (option_*), which
	// fire on admin config changes rather than per-resource CRUD.
	{Name: "events", Type: core.ConnectionTypeMultiSelect, Label: "Events", Required: true, Options: []core.ConnectionOption{
		{Name: "Created", Value: "jira:issue_created", Group: "Issue"},
		{Name: "Updated", Value: "jira:issue_updated", Group: "Issue"},
		{Name: "Deleted", Value: "jira:issue_deleted", Group: "Issue"},
		{Name: "Created", Value: "comment_created", Group: "Comment"},
		{Name: "Updated", Value: "comment_updated", Group: "Comment"},
		{Name: "Deleted", Value: "comment_deleted", Group: "Comment"},
		{Name: "Created", Value: "worklog_created", Group: "Worklog"},
		{Name: "Updated", Value: "worklog_updated", Group: "Worklog"},
		{Name: "Deleted", Value: "worklog_deleted", Group: "Worklog"},
		{Name: "Created", Value: "issuelink_created", Group: "Issue Link"},
		{Name: "Deleted", Value: "issuelink_deleted", Group: "Issue Link"},
		{Name: "Created", Value: "jira:version_created", Group: "Version"},
		{Name: "Updated", Value: "jira:version_updated", Group: "Version"},
		{Name: "Deleted", Value: "jira:version_deleted", Group: "Version"},
		{Name: "Released", Value: "jira:version_released", Group: "Version"},
		{Name: "Unreleased", Value: "jira:version_unreleased", Group: "Version"},
		{Name: "Moved", Value: "jira:version_moved", Group: "Version"},
		{Name: "Created", Value: "sprint_created", Group: "Sprint"},
		{Name: "Updated", Value: "sprint_updated", Group: "Sprint"},
		{Name: "Deleted", Value: "sprint_deleted", Group: "Sprint"},
		{Name: "Started", Value: "sprint_started", Group: "Sprint"},
		{Name: "Closed", Value: "sprint_closed", Group: "Sprint"},
		{Name: "Created", Value: "board_created", Group: "Board"},
		{Name: "Updated", Value: "board_updated", Group: "Board"},
		{Name: "Deleted", Value: "board_deleted", Group: "Board"},
		{Name: "Configuration Changed", Value: "board_configuration_changed", Group: "Board"},
		{Name: "Created", Value: "project_created", Group: "Project"},
		{Name: "Updated", Value: "project_updated", Group: "Project"},
		{Name: "Deleted", Value: "project_deleted", Group: "Project"},
		{Name: "Created", Value: "user_created", Group: "User"},
		{Name: "Updated", Value: "user_updated", Group: "User"},
		{Name: "Deleted", Value: "user_deleted", Group: "User"},
		{Name: "Voting Changed", Value: "option_voting_changed", Group: "Configuration"},
		{Name: "Watching Changed", Value: "option_watching_changed", Group: "Configuration"},
		{Name: "Unassigned Issues Changed", Value: "option_unassigned_issues_changed", Group: "Configuration"},
		{Name: "Subtasks Changed", Value: "option_subtasks_changed", Group: "Configuration"},
		{Name: "Attachments Changed", Value: "option_attachments_changed", Group: "Configuration"},
		{Name: "Issue Links Changed", Value: "option_issuelinks_changed", Group: "Configuration"},
		{Name: "Time Tracking Changed", Value: "option_timetracking_changed", Group: "Configuration"},
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

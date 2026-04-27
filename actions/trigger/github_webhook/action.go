package github_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Webhook Trigger"
	Description  = "Triggers a flow when a GitHub webhook event is received"
	Website      = "https://www.flomation.co"
	Icon         = "github"
	Date         = "26/04/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "webhook_secret", Type: core.ConnectionTypeString, Label: "Webhook Secret", Placeholder: "HMAC secret for X-Hub-Signature-256 validation", Required: true},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Event Filter", Placeholder: "Comma-separated: push,pull_request,issue_comment,workflow_run"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "action", Type: core.ConnectionTypeString, Label: "Action"},
	{Name: "repository_name", Type: core.ConnectionTypeString, Label: "Repository Name"},
	{Name: "repository_full_name", Type: core.ConnectionTypeString, Label: "Repository Full Name"},
	{Name: "repository_url", Type: core.ConnectionTypeString, Label: "Repository URL"},
	{Name: "sender_login", Type: core.ConnectionTypeString, Label: "Sender Login"},
	{Name: "ref", Type: core.ConnectionTypeString, Label: "Ref"},
	{Name: "pull_request_number", Type: core.ConnectionTypeString, Label: "Pull Request Number"},
	{Name: "pull_request_title", Type: core.ConnectionTypeString, Label: "Pull Request Title"},
	{Name: "pull_request_state", Type: core.ConnectionTypeString, Label: "Pull Request State"},
	{Name: "workflow_run_id", Type: core.ConnectionTypeString, Label: "Workflow Run ID"},
	{Name: "workflow_run_status", Type: core.ConnectionTypeString, Label: "Workflow Run Status"},
	{Name: "comment_body", Type: core.ConnectionTypeString, Label: "Comment Body"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed
// as outputs — they contain secrets or internal filter settings.
var configInputs = map[string]bool{
	"webhook_secret": true,
	"event_filter":   true,
	"__node_id":      true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing GitHub webhook trigger")

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
	eventType := str(data["event_type"])
	action := str(data["action"])
	repo := str(data["repository_full_name"])
	sender := str(data["sender_login"])
	ref := str(data["ref"])

	switch eventType {
	case "push":
		return fmt.Sprintf("[GitHub Push] %s pushed to %s on %s", sender, ref, repo)

	case "pull_request":
		number := str(data["pull_request_number"])
		title := str(data["pull_request_title"])
		state := str(data["pull_request_state"])
		return fmt.Sprintf("[GitHub PR] %s %s pull request #%s \"%s\" [%s] on %s",
			sender, action, number, title, state, repo)

	case "pull_request_review":
		number := str(data["pull_request_number"])
		return fmt.Sprintf("[GitHub Review] %s %s a review on PR #%s on %s",
			sender, action, number, repo)

	case "issue_comment":
		comment := str(data["comment_body"])
		if len(comment) > 200 {
			comment = comment[:200] + "..."
		}
		number := str(data["pull_request_number"])
		if number != "" {
			return fmt.Sprintf("[GitHub Comment] %s commented on PR #%s on %s: %s",
				sender, number, repo, comment)
		}
		return fmt.Sprintf("[GitHub Comment] %s commented on %s: %s", sender, repo, comment)

	case "workflow_run":
		runID := str(data["workflow_run_id"])
		status := str(data["workflow_run_status"])
		return fmt.Sprintf("[GitHub Workflow] Workflow run #%s is %s (%s) on %s",
			runID, status, action, repo)

	default:
		if action != "" {
			return fmt.Sprintf("[GitHub Event] %s %s on %s by %s", eventType, action, repo, sender)
		}
		return fmt.Sprintf("[GitHub Event] %s on %s by %s", eventType, repo, sender)
	}
}

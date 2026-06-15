package gitlab_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Webhook Trigger"
	Description  = "Triggers a flow when a GitLab webhook event is received"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab"
	Date         = "26/04/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "webhook_secret", Type: core.ConnectionTypeSecret, Label: "Webhook Secret Token", Placeholder: "Shared secret for X-Gitlab-Token validation", Required: true},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Event Filter", Placeholder: "Comma-separated: push,merge_request,note,pipeline,tag_push"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "object_kind", Type: core.ConnectionTypeString, Label: "Object Kind"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID"},
	{Name: "project_name", Type: core.ConnectionTypeString, Label: "Project Name"},
	{Name: "project_url", Type: core.ConnectionTypeString, Label: "Project URL"},
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name"},
	{Name: "user_username", Type: core.ConnectionTypeString, Label: "User Username"},
	{Name: "ref", Type: core.ConnectionTypeString, Label: "Ref"},
	{Name: "merge_request_iid", Type: core.ConnectionTypeString, Label: "Merge Request IID"},
	{Name: "merge_request_title", Type: core.ConnectionTypeString, Label: "Merge Request Title"},
	{Name: "merge_request_state", Type: core.ConnectionTypeString, Label: "Merge Request State"},
	{Name: "merge_request_action", Type: core.ConnectionTypeString, Label: "Merge Request Action"},
	{Name: "pipeline_id", Type: core.ConnectionTypeString, Label: "Pipeline ID"},
	{Name: "pipeline_status", Type: core.ConnectionTypeString, Label: "Pipeline Status"},
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
	log.Debug("Executing GitLab webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	// Synthesise a human-readable content summary so the AI node's prompt
	// auto-wire receives a useful description of what happened.
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
	kind := str(data["object_kind"])
	project := str(data["project_name"])
	user := str(data["user_name"])
	ref := str(data["ref"])

	switch kind {
	case "push":
		return fmt.Sprintf("[GitLab Push] %s pushed to %s on project %s (ID: %s)",
			user, ref, project, str(data["project_id"]))

	case "merge_request":
		action := str(data["merge_request_action"])
		iid := str(data["merge_request_iid"])
		title := str(data["merge_request_title"])
		state := str(data["merge_request_state"])
		return fmt.Sprintf("[GitLab MR] %s %s merge request !%s \"%s\" [%s] on project %s (ID: %s)",
			user, action, iid, title, state, project, str(data["project_id"]))

	case "note":
		comment := str(data["comment_body"])
		if len(comment) > 200 {
			comment = comment[:200] + "..."
		}
		mrIID := str(data["merge_request_iid"])
		if mrIID != "" {
			return fmt.Sprintf("[GitLab Comment] %s commented on MR !%s on project %s: %s",
				user, mrIID, project, comment)
		}
		return fmt.Sprintf("[GitLab Comment] %s commented on project %s: %s",
			user, project, comment)

	case "pipeline":
		pipelineID := str(data["pipeline_id"])
		status := str(data["pipeline_status"])
		return fmt.Sprintf("[GitLab Pipeline] Pipeline #%s is %s on %s for project %s (ID: %s)",
			pipelineID, status, ref, project, str(data["project_id"]))

	case "tag_push":
		return fmt.Sprintf("[GitLab Tag] %s pushed tag %s on project %s (ID: %s)",
			user, ref, project, str(data["project_id"]))

	default:
		eventType := str(data["event_type"])
		if eventType == "" {
			eventType = kind
		}
		return fmt.Sprintf("[GitLab Event] %s event on project %s by %s",
			eventType, project, user)
	}
}

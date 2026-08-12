package heygen_webhook

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "HeyGen Webhook Trigger"
	Description  = "Triggers a flow when a HeyGen video is ready (avatar_video.success / .fail, translation events)."
	Website      = "https://www.flomation.co"
	Icon         = "film+bolt"
	Date         = "12/08/2026"
	Type         = core.ActionTypeTrigger
)

// HeyGen delivers completion events to the callback_url you pass to a generate
// action. Rather than depend on HeyGen's (endpoint-scoped) signing secret, this
// trigger uses the same defence-in-depth model as the Apollo trigger: the
// author sets a `webhook_secret` and appends it to the callback URL as
// `?secret=…`. Launch requires it (query param or X-Flomation-Webhook-Secret
// header) and compares it in constant time, on top of the unguessable
// /webhook/:id route.
var Inputs = [...]core.Connection{
	{Name: "webhook_secret", Type: core.ConnectionTypeSecret, Label: "Webhook Secret", Placeholder: "A secret token you choose; append it to the callback URL as ?secret=…", Required: true},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Event Filter", Placeholder: "Comma-separated event types, e.g. avatar_video.success (blank = all)"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "video_id", Type: core.ConnectionTypeString, Label: "Video ID"},
	{Name: "video_url", Type: core.ConnectionTypeString, Label: "Video URL"},
	{Name: "callback_id", Type: core.ConnectionTypeString, Label: "Callback ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they hold the shared secret and the event filter.
var configInputs = map[string]bool{
	"webhook_secret": true,
	"event_filter":   true,
	"__node_id":      true,
}

// Execute echoes the event data Launch resolved from the verified HeyGen webhook
// into the flow's outputs. Launch does the secret-token check and JSON parsing;
// this node just surfaces the fields.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing HeyGen webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	if _, ok := result["content"]; !ok {
		if et, ok := result["event_type"].(string); ok && et != "" {
			result["content"] = "HeyGen event: " + et
		}
	}
	return result, nil
}

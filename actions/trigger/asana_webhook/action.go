package asana_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Asana Webhook Trigger"
	Description  = "Triggers a flow when something changes on an Asana task or project you watch (a task is added, changed, completed, commented on, and so on). The webhook is registered automatically for the resource you choose — Asana's handshake and per-delivery HMAC-SHA256 signature are handled for you."
	Website      = "https://www.flomation.co"
	Icon         = "asana"
	Date         = "07/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "workspace", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "The workspace that owns the resource (used to load the resource picker)"},
	{Name: "resource", Type: core.ConnectionTypeString, Label: "Watch", Placeholder: "The task or project ID to watch for changes", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "action", Type: core.ConnectionTypeString, Label: "Action"},
	{Name: "resource_id", Type: core.ConnectionTypeString, Label: "Resource ID"},
	{Name: "resource_type", Type: core.ConnectionTypeString, Label: "Resource Type"},
	{Name: "user", Type: core.ConnectionTypeString, Label: "User"},
	{Name: "created_at", Type: core.ConnectionTypeString, Label: "Timestamp"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they carry the credential or the watched-resource setting. The
// launch service injects the event fields (action, resource_id, …) at fire time,
// and Execute echoes those through to the outputs.
var configInputs = map[string]bool{
	"access_token": true,
	"workspace":    true,
	"resource":     true,
	"__node_id":    true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Asana webhook trigger")

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
	action := str(data["action"])
	if action == "" {
		action = "change"
	}
	rt := str(data["resource_type"])
	if rt != "" {
		return fmt.Sprintf("[Asana] %s on %s", action, rt)
	}
	return fmt.Sprintf("[Asana] %s", action)
}

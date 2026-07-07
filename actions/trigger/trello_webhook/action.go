package trello_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Trello Webhook Trigger"
	Description  = "Triggers a flow when something changes on a Trello board, list, or card you watch (a card is created, moved, commented on, and so on). The webhook is registered automatically for the model you choose; set the Trello API Secret to have each delivery's HMAC-SHA1 signature verified."
	Website      = "https://www.flomation.co"
	Icon         = "trello"
	Date         = "07/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "model_id", Type: core.ConnectionTypeString, Label: "Watch", Placeholder: "The board (or list/card ID) to watch for changes", Required: true},
	{Name: "secret", Type: core.ConnectionTypeSecret, Label: "API Secret", Placeholder: "Optional — your Trello API Secret, used to verify each delivery's HMAC-SHA1 signature"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "action_type", Type: core.ConnectionTypeString, Label: "Action Type"},
	{Name: "action_id", Type: core.ConnectionTypeString, Label: "Action ID"},
	{Name: "model_id", Type: core.ConnectionTypeString, Label: "Model ID"},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board ID"},
	{Name: "card_id", Type: core.ConnectionTypeString, Label: "Card ID"},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "List ID"},
	{Name: "member", Type: core.ConnectionTypeString, Label: "Member"},
	{Name: "date", Type: core.ConnectionTypeString, Label: "Timestamp"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they carry the credentials or the watched-model setting. The launch
// service injects the event fields (action_type, board_id, …) at fire time, and
// Execute echoes those through to the outputs.
var configInputs = map[string]bool{
	"api_key":   true,
	"api_token": true,
	"model_id":  true,
	"secret":    true,
	"__node_id": true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Trello webhook trigger")

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
	action := str(data["action_type"])
	if action == "" {
		action = "change"
	}
	if card := str(data["card_id"]); card != "" {
		return fmt.Sprintf("[Trello] %s on card %s", action, card)
	}
	return fmt.Sprintf("[Trello] %s", action)
}

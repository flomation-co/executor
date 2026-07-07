package monday_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Monday.com Webhook Trigger"
	Description  = "Triggers a flow when something happens on a Monday.com board you watch — an item is created, a column value changes, an update is posted, and so on. The webhook is registered automatically for the board and event you choose (Monday's challenge handshake is handled for you)."
	Website      = "https://www.flomation.co"
	Icon         = "monday"
	Date         = "07/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board to watch for changes", Required: true},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event", Placeholder: "Which event to fire on", Required: true, Options: []core.ConnectionOption{
		{Name: "Item Created", Value: "create_item"},
		{Name: "Item Name Changed", Value: "change_name"},
		{Name: "Column Value Changed", Value: "change_column_value"},
		{Name: "Status Column Changed", Value: "change_status_column_value"},
		{Name: "Update Posted", Value: "create_update"},
		{Name: "Item Archived", Value: "item_archived"},
		{Name: "Item Deleted", Value: "item_deleted"},
		{Name: "Item Moved to Group", Value: "item_moved_to_any_group"},
		{Name: "Subitem Created", Value: "create_subitem"},
		{Name: "Column Created", Value: "create_column"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board ID"},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID"},
	{Name: "column_id", Type: core.ConnectionTypeString, Label: "Column ID"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they carry the credential or the watched-event setting. The launch
// service injects the event fields (event_type, item_id, board_id, …) at fire
// time.
//
// NOTE: board_id is deliberately NOT listed here even though it is a config
// input, because it is ALSO a declared output that launch fills from the event.
// Listing it would make Execute skip the injected value and leave the board_id
// output empty. The watched board equals the event's board, so echoing it is
// correct either way.
var configInputs = map[string]bool{
	"api_token": true,
	"event":     true,
	"__node_id": true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Monday.com webhook trigger")

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
	event := str(data["event_type"])
	if event == "" {
		event = "event"
	}
	if item := str(data["item_id"]); item != "" {
		return fmt.Sprintf("[Monday] %s on item %s", event, item)
	}
	return fmt.Sprintf("[Monday] %s", event)
}

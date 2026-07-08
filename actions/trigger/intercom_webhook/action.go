package intercom_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom Webhook Trigger"
	Description  = "Triggers a flow when something happens in your Intercom workspace — a new conversation starts, a customer replies, a ticket changes state, a contact is created, and so on. Intercom doesn't let apps register webhooks automatically, so paste the trigger's webhook URL into your Intercom Developer Hub app under Configure → Webhooks and pick the topics to send."
	Website      = "https://www.flomation.co"
	Icon         = "intercom"
	Date         = "08/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App client secret — verifies webhook signatures (recommended)"},
	{Name: "topic_filter", Type: core.ConnectionTypeString, Label: "Only Fire On", Placeholder: "Which Intercom event to fire on (default: all)", Options: []core.ConnectionOption{
		{Name: "All events", Value: ""},
		{Name: "New conversation started", Value: "conversation.user.created"},
		{Name: "Customer replied to conversation", Value: "conversation.user.replied"},
		{Name: "Teammate replied to conversation", Value: "conversation.admin.replied"},
		{Name: "Conversation assigned", Value: "conversation.admin.assigned"},
		{Name: "Conversation closed", Value: "conversation.admin.closed"},
		{Name: "Conversation opened", Value: "conversation.admin.opened"},
		{Name: "Conversation snoozed", Value: "conversation.admin.snoozed"},
		{Name: "Note added to conversation", Value: "conversation.admin.noted"},
		{Name: "Conversation rated", Value: "conversation.rating.added"},
		{Name: "Conversation priority changed", Value: "conversation.priority.updated"},
		{Name: "Contact created (user)", Value: "contact.user.created"},
		{Name: "Lead created", Value: "contact.lead.created"},
		{Name: "Contact updated (user)", Value: "contact.user.updated"},
		{Name: "Lead updated", Value: "contact.lead.updated"},
		{Name: "Lead signed up (became a user)", Value: "contact.lead.signed_up"},
		{Name: "Contact email updated", Value: "contact.email.updated"},
		{Name: "Contacts merged", Value: "contact.merged"},
		{Name: "Contact deleted", Value: "contact.deleted"},
		{Name: "Contact archived", Value: "contact.archived"},
		{Name: "Contact subscribed to emails", Value: "contact.subscribed"},
		{Name: "Contact unsubscribed from emails", Value: "contact.unsubscribed"},
		{Name: "Ticket created", Value: "ticket.created"},
		{Name: "Ticket state changed", Value: "ticket.state.updated"},
		{Name: "Ticket closed", Value: "ticket.closed"},
		{Name: "Ticket assigned to teammate", Value: "ticket.admin.assigned"},
		{Name: "Ticket assigned to team", Value: "ticket.team.assigned"},
		{Name: "Teammate replied to ticket", Value: "ticket.admin.replied"},
		{Name: "Customer replied to ticket", Value: "ticket.contact.replied"},
		{Name: "Note added to ticket", Value: "ticket.note.created"},
		{Name: "Ticket rated", Value: "ticket.rating.provided"},
		{Name: "Company created", Value: "company.created"},
		{Name: "Company updated", Value: "company.updated"},
		{Name: "Company deleted", Value: "company.deleted"},
		{Name: "Article published", Value: "article.published"},
		{Name: "Event tracked", Value: "event.created"},
		{Name: "Visitor signed up", Value: "visitor.signed_up"},
		{Name: "Teammate away mode changed", Value: "admin.away_mode_updated"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic"},
	{Name: "item_type", Type: core.ConnectionTypeString, Label: "Item Type"},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID"},
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID"},
	{Name: "ticket_id", Type: core.ConnectionTypeString, Label: "Ticket ID"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID"},
	{Name: "company_id", Type: core.ConnectionTypeString, Label: "Company ID"},
	{Name: "admin_id", Type: core.ConnectionTypeString, Label: "Admin ID"},
	{Name: "app_id", Type: core.ConnectionTypeString, Label: "App ID"},
	{Name: "created_at", Type: core.ConnectionTypeString, Label: "Created At (Unix)"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they carry the signing secret and the topic-filter setting. The
// launch service injects the event fields (topic, item_type, item_id, …) at
// fire time. Note topic_filter (the config) and topic (the output) are
// deliberately different names, so there is no board_id-style overlap here.
var configInputs = map[string]bool{
	"client_secret": true,
	"topic_filter":  true,
	"__node_id":     true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Intercom webhook trigger")

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
	topic := str(data["topic"])
	if topic == "" {
		topic = "event"
	}
	if itemType, itemID := str(data["item_type"]), str(data["item_id"]); itemType != "" && itemID != "" {
		return fmt.Sprintf("[Intercom] %s on %s %s", topic, itemType, itemID)
	}
	return fmt.Sprintf("[Intercom] %s", topic)
}

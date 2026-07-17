package messagebrokers_azureservicebus_deadletter_receive

import (
	"fmt"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Receive Dead-Lettered"
	Description  = "Receive from the dead-letter queue of a queue or subscription, exposing the dead-letter reason and description that say why each message ended up there. Messages arrive here automatically once delivery attempts exceed MaxDeliveryCount (default 10), or on expiry. A dead-letter queue has no dead-letter queue of its own, so abandoning here simply redelivers. Settlement happens HERE, in this node, because a lock token belongs to the AMQP connection that took it — a downstream Complete node is impossible, and when this node's link closes the broker releases anything unsettled immediately. Defer is the one handoff that survives: its sequence numbers are durable, so another flow can pick the message up with Receive Deferred Messages."
	Website      = "https://www.flomation.co"
	Icon         = "azure+triangle-exclamation"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "connection_string", Type: core.ConnectionTypeSecret, Label: "Connection String", Placeholder: "Endpoint=sb://myns.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=… — the NAMESPACE-level policy, not a queue's", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connection_string"}}},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connection String", Value: "connection_string"}, {Name: "Microsoft Entra (service principal)", Value: "entra"}}},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "myns.servicebus.windows.net — the host only, no sb:// prefix", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs an Azure Service Bus Data role on the namespace — subscription Owner is not enough", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "entity_type", Type: core.ConnectionTypeString, Label: "Entity Type", Options: []core.ConnectionOption{{Name: "Queue", Value: "queue"}, {Name: "Topic Subscription", Value: "subscription"}}},
	{Name: "queue", Type: core.ConnectionTypeString, Label: "Queue", Placeholder: "orders", Visible: &core.VisibleWhen{Field: "entity_type", Values: []string{"", "queue"}}},
	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic", Placeholder: "order-events", Visible: &core.VisibleWhen{Field: "entity_type", Values: []string{"subscription"}}},
	{Name: "subscription", Type: core.ConnectionTypeString, Label: "Subscription", Placeholder: "billing", Visible: &core.VisibleWhen{Field: "entity_type", Values: []string{"subscription"}}},
	{Name: "sub_queue", Type: core.ConnectionTypeString, Label: "Which Dead-Letter Queue", Options: []core.ConnectionOption{{Name: "Dead-Letter (the usual one)", Value: "dead_letter"}, {Name: "Transfer Dead-Letter (auto-forwarding failures)", Value: "transfer"}}},
	{Name: "max_messages", Type: core.ConnectionTypeInteger, Label: "Max Messages", Placeholder: "1 (max 100) — this is 'up to', never 'wait for': receiving fewer is normal"},
	{Name: "max_wait_seconds", Type: core.ConnectionTypeInteger, Label: "Max Wait (seconds)", Placeholder: "10 (max 300) — the ceiling on an empty queue; returns the moment anything arrives"},
	{Name: "receive_mode", Type: core.ConnectionTypeString, Label: "Receive Mode", Options: []core.ConnectionOption{{Name: "Peek-Lock (locks the message, settled below)", Value: "peek_lock"}, {Name: "Receive and Delete (destructive — gone before the flow sees it)", Value: "receive_and_delete"}}},
	{Name: "disposition", Type: core.ConnectionTypeString, Label: "Then", Options: []core.ConnectionOption{{Name: "Complete — remove from the queue", Value: "complete"}, {Name: "Abandon — release for immediate redelivery", Value: "abandon"}, {Name: "Dead-Letter — move to the dead-letter queue", Value: "dead_letter"}, {Name: "Defer — keep, retrievable later by sequence number", Value: "defer"}}, Visible: &core.VisibleWhen{Field: "receive_mode", Values: []string{"", "peek_lock"}}},
	{Name: "dead_letter_reason", Type: core.ConnectionTypeString, Label: "Dead-Letter Reason", Placeholder: "ValidationFailed", Visible: &core.VisibleWhen{Field: "disposition", Values: []string{"dead_letter"}}},
	{Name: "dead_letter_error_description", Type: core.ConnectionTypeString, Label: "Dead-Letter Description", Placeholder: "Order had no customer reference", Visible: &core.VisibleWhen{Field: "disposition", Values: []string{"dead_letter"}}},
	{Name: "parse_json", Type: core.ConnectionTypeBoolean, Label: "Parse JSON Body", Placeholder: "Also expose the body decoded, as body_json"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Messages"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "received", Type: core.ConnectionTypeBoolean, Label: "Received Any"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sb.GetAuth(inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	spec, err := sb.DeadLetterSpec(inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	mode, err := sb.ReceiveModeFrom(inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	disposition, err := sb.DispositionFrom(inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	spec.Mode = mode

	receiver, err := sb.NewReceiver(auth, spec)
	if err != nil {
		return sb.Fail(auth, "Could not open a receiver", err), nil
	}
	ctx := sb.Context(flow)
	defer func() { _ = receiver.Close(ctx) }()

	msgs, err := sb.ReceiveWithin(ctx, receiver, sb.ClampMaxMessages(inputs), sb.ClampMaxWait(inputs))
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not receive from dead-letter queue of %s", spec.Entity()), err), nil
	}
	if err := sb.Settle(ctx, receiver, msgs, mode, disposition,
		sb.OptionalString("dead_letter_reason", inputs),
		sb.OptionalString("dead_letter_error_description", inputs)); err != nil {
		return sb.Fail(auth, "Received messages but could not settle them", err), nil
	}

	return sb.MessagesResult(sb.MessagesOutput(msgs, sb.OptionalBool("parse_json", inputs)), "Received", "dead-letter queue of "+spec.Entity()), nil
}

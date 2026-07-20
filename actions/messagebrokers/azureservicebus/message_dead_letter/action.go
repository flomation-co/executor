package messagebrokers_azureservicebus_message_dead_letter

import (
	"fmt"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	azservicebus "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Dead-Letter Messages"
	Description  = "Receive messages and dead-letter all of them with a reason and description — the receive-inspect-reject shape, in one step. This is the only dead-letter action that is not a disposition on a receive, and it still receives first for the same reason: a lock token cannot cross nodes, so there is nothing a standalone reject node could hold on to. To dead-letter conditionally, use Receive from Queue with Then = Dead-Letter behind a filter."
	Website      = "https://www.flomation.co"
	Icon         = "azure+ban"
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
	{Name: "max_messages", Type: core.ConnectionTypeInteger, Label: "Max Messages", Placeholder: "1 (max 100) — every message received here is dead-lettered"},
	{Name: "max_wait_seconds", Type: core.ConnectionTypeInteger, Label: "Max Wait (seconds)", Placeholder: "10 (max 300)"},
	{Name: "dead_letter_reason", Type: core.ConnectionTypeString, Label: "Reason", Placeholder: "ValidationFailed"},
	{Name: "dead_letter_error_description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Order had no customer reference"},
	{Name: "parse_json", Type: core.ConnectionTypeBoolean, Label: "Parse JSON Body", Placeholder: "Also expose the body decoded, as body_json"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Dead-Lettered Messages"},
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
	spec, err := sb.EntitySpec(inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	// Peek-lock is not a choice here: a ReceiveAndDelete message has no lock
	// to dead-letter with, and the SDK rejects settling it outright.
	spec.Mode = azservicebus.ReceiveModePeekLock

	receiver, err := sb.NewReceiver(auth, spec)
	if err != nil {
		return sb.Fail(auth, "Could not open a receiver", err), nil
	}
	ctx := sb.Context(flow)
	defer func() { _ = receiver.Close(ctx) }()

	msgs, err := sb.ReceiveWithin(ctx, receiver, sb.ClampMaxMessages(inputs), sb.ClampMaxWait(inputs))
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not receive from %s", spec.Entity()), err), nil
	}
	if err := sb.Settle(ctx, receiver, msgs, spec.Mode, sb.DispositionDeadLetter,
		sb.OptionalString("dead_letter_reason", inputs),
		sb.OptionalString("dead_letter_error_description", inputs)); err != nil {
		return sb.Fail(auth, "Received messages but could not dead-letter them", err), nil
	}
	return sb.MessagesResult(sb.MessagesOutput(msgs, sb.OptionalBool("parse_json", inputs)), "Dead-lettered", spec.Entity()), nil
}

package messagebrokers_azureservicebus_deadletter_peek

import (
	"fmt"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	azservicebus "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Peek Dead-Lettered"
	Description  = "Look at dead-lettered messages without consuming them — diagnosis without commitment. Pairs with Receive Dead-Lettered once you know what is in there."
	Website      = "https://www.flomation.co"
	Icon         = "azure+circle-exclamation"
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
	{Name: "max_messages", Type: core.ConnectionTypeInteger, Label: "Max Messages", Placeholder: "1 (max 100)"},
	{Name: "from_sequence_number", Type: core.ConnectionTypeString, Label: "From Sequence Number", Placeholder: "Blank to continue from the last peek"},
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

	receiver, err := sb.NewReceiver(auth, spec)
	if err != nil {
		return sb.Fail(auth, "Could not open a receiver", err), nil
	}
	ctx := sb.Context(flow)
	defer func() { _ = receiver.Close(ctx) }()

	opts := &azservicebus.PeekMessagesOptions{}
	if raw := sb.OptionalString("from_sequence_number", inputs); raw != "" {
		nums, err := sb.ParseSequenceNumbers(raw)
		if err != nil {
			return sb.ErrorResult(err.Error()), nil
		}
		opts.FromSequenceNumber = &nums[0]
	}

	msgs, err := receiver.PeekMessages(ctx, sb.ClampMaxMessages(inputs), opts)
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not peek dead-letter queue of %s", spec.Entity()), err), nil
	}
	return sb.MessagesResult(sb.MessagesOutput(msgs, sb.OptionalBool("parse_json", inputs)), "Peeked", "dead-letter queue of "+spec.Entity()), nil
}

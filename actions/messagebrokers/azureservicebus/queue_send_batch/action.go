package messagebrokers_azureservicebus_queue_send_batch

import (
	"errors"
	"fmt"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	azservicebus "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Send Batch to Queue"
	Description  = "Send many messages to a queue in one AMQP transfer. Takes a JSON array of message objects ({\"body\":…, \"message_id\":…}) or plain strings. Batching is size-aware: the message that does not fit the 256KB envelope is named rather than silently dropped."
	Website      = "https://www.flomation.co"
	Icon         = "azure+envelopes-bulk"
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
	{Name: "queue", Type: core.ConnectionTypeString, Label: "Queue", Placeholder: "orders — the queue name on its own, not a URL", Required: true},
	{Name: "messages", Type: core.ConnectionTypeObject, Label: "Messages (JSON array)", Placeholder: `[{"body":"first"},{"body":"second","subject":"order.created"}] — objects or plain-string bodies`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Sent Messages"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sb.GetAuth(inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	queue, err := sb.RequiredString("queue", inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	raw, err := sb.OptionalJSON("messages", inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	list, ok := raw.([]interface{})
	if !ok || len(list) == 0 {
		return sb.ErrorResult(`messages must be a non-empty JSON array, e.g. [{"body":"first"},{"body":"second"}]`), nil
	}

	msgs := make([]*azservicebus.Message, 0, len(list))
	echoes := make([]interface{}, 0, len(list))
	for i, item := range list {
		msg, err := sb.MessageFromJSON(i, item)
		if err != nil {
			return sb.ErrorResult(err.Error()), nil
		}
		msgs = append(msgs, msg)
		echoes = append(echoes, sb.MessageEcho(msg, queue))
	}

	sender, err := sb.NewSender(auth, queue)
	if err != nil {
		return sb.Fail(auth, "Could not open a sender", err), nil
	}
	ctx := sb.Context(flow)
	defer func() { _ = sender.Close(ctx) }()

	batch, err := sender.NewMessageBatch(ctx, nil)
	if err != nil {
		return sb.Fail(auth, "Could not start a batch", err), nil
	}
	for i, msg := range msgs {
		if err := batch.AddMessage(msg, nil); err != nil {
			// AddMessage measures the encoded message against the remaining
			// envelope, so this is the ONE place we can name which message
			// broke the cap. Reporting it as a plain send failure would leave
			// the operator bisecting the array by hand.
			if errors.Is(err, azservicebus.ErrMessageTooLarge) {
				return sb.ErrorResult(fmt.Sprintf("messages[%d] does not fit the batch: a batch is capped at 256KB on Standard (100MB on Premium), and that covers headers and application properties as well as bodies. Send it on its own, or split the array — %d message(s) had already been added.", i, batch.NumMessages())), nil
			}
			return sb.Fail(auth, fmt.Sprintf("Could not add messages[%d] to the batch", i), err), nil
		}
	}

	if err := sender.SendMessageBatch(ctx, batch, nil); err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not send the batch to queue %s", queue), err), nil
	}
	return sb.ListResult(echoes, fmt.Sprintf("Sent %d message(s) to queue %s in one batch", len(echoes), queue)), nil
}

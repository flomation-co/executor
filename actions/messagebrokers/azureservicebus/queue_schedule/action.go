package messagebrokers_azureservicebus_queue_schedule

import (
	"fmt"
	"strconv"
	"time"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	azservicebus "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Schedule Message"
	Description  = "Schedule a message to be enqueued on a queue at a future time. Outputs the sequence numbers the broker assigns — they are the ONLY handle Cancel Scheduled Messages accepts, so a scheduled message whose sequence number is not kept can never be cancelled."
	Website      = "https://www.flomation.co"
	Icon         = "azure+clock"
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
	{Name: "scheduled_enqueue_time", Type: core.ConnectionTypeString, Label: "Scheduled Enqueue Time", Placeholder: "2026-07-17T09:30:00Z — RFC3339, in the future", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Body", Placeholder: `{"order":123} — text or JSON. The 256KB Standard-tier cap covers headers and properties too, not just this`, Required: true},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type", Placeholder: "application/json"},
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID", Placeholder: "Free-form. Only de-duplicates if the queue was CREATED with duplicate detection — the flag cannot be added later"},
	{Name: "session_id", Type: core.ConnectionTypeString, Label: "Session ID", Placeholder: "Required if the entity was created with sessions; ignored if it was not"},
	{Name: "correlation_id", Type: core.ConnectionTypeString, Label: "Correlation ID", Placeholder: "Usually the Message ID this is a reply to"},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "order.created — like an email subject line; correlation filters match on it"},
	{Name: "reply_to", Type: core.ConnectionTypeString, Label: "Reply To", Placeholder: "The queue a reply should be sent to"},
	{Name: "to", Type: core.ConnectionTypeString, Label: "To", Placeholder: "Advisory only — Service Bus does not route on this"},
	{Name: "partition_key", Type: core.ConnectionTypeString, Label: "Partition Key", Placeholder: "Keeps related messages on one partition; ignored on a non-partitioned entity"},
	{Name: "time_to_live_seconds", Type: core.ConnectionTypeInteger, Label: "Time To Live (seconds)", Placeholder: "Blank for the entity's default. A longer TTL than the entity's is silently shortened to it"},
	{Name: "application_properties", Type: core.ConnectionTypeObject, Label: "Application Properties (JSON)", Placeholder: `{"tenant":"acme"} — custom metadata; counts towards the size cap`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Sequence Number"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Scheduled Message"},
	{Name: "sequence_numbers", Type: core.ConnectionTypeObject, Label: "Sequence Numbers (keep these — cancelling needs them)"},
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
	rawTime, err := sb.RequiredString("scheduled_enqueue_time", inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	at, err := sb.ParseTime("scheduled_enqueue_time", rawTime)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	msg, err := sb.BuildMessage(inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}

	sender, err := sb.NewSender(auth, queue)
	if err != nil {
		return sb.Fail(auth, "Could not open a sender", err), nil
	}
	ctx := sb.Context(flow)
	defer func() { _ = sender.Close(ctx) }()

	seqs, err := sender.ScheduleMessages(ctx, []*azservicebus.Message{msg}, at, nil)
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not schedule a message on queue %s", queue), err), nil
	}
	if len(seqs) == 0 {
		return sb.ErrorResult("Service Bus accepted the schedule but returned no sequence number, so the message cannot be cancelled later — treat it as sent"), nil
	}

	nums := make([]interface{}, 0, len(seqs))
	for _, s := range seqs {
		nums = append(nums, s)
	}
	result := sb.MessageEcho(msg, queue)
	result["sequence_numbers"] = nums
	result["scheduled_enqueue_time"] = at.UTC().Format(time.RFC3339)

	out := sb.ResourceResult(strconv.FormatInt(seqs[0], 10), result,
		fmt.Sprintf("Scheduled 1 message on queue %s for %s (sequence number %d)", queue, at.UTC().Format(time.RFC3339), seqs[0]))
	out["sequence_numbers"] = nums
	return out, nil
}

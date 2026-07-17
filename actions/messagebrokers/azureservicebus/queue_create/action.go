package messagebrokers_azureservicebus_queue_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Create Queue"
	Description  = "Create a queue. Requires Session, Requires Duplicate Detection and Enable Partitioning are IMMUTABLE — they can only be set at creation, and changing your mind later means recreating the queue and losing its messages. Lock Duration cannot exceed 5 minutes whatever is asked for."
	Website      = "https://www.flomation.co"
	Icon         = "azure+plus"
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
	{Name: "lock_duration_seconds", Type: core.ConnectionTypeInteger, Label: "Lock Duration (seconds)", Placeholder: "60 — how long a peek-lock holds. Capped at 300 by Service Bus; it cannot be raised further"},
	{Name: "max_delivery_count", Type: core.ConnectionTypeInteger, Label: "Max Delivery Count", Placeholder: "10 — attempts before a message is dead-lettered automatically"},
	{Name: "default_message_time_to_live_seconds", Type: core.ConnectionTypeInteger, Label: "Default Message TTL (seconds)", Placeholder: "Blank for the maximum (about 10675199 days)"},
	{Name: "requires_session", Type: core.ConnectionTypeBoolean, Label: "Requires Session (IMMUTABLE)", Placeholder: "FIFO per session. Cannot be changed after creation — the queue must be recreated"},
	{Name: "requires_duplicate_detection", Type: core.ConnectionTypeBoolean, Label: "Requires Duplicate Detection (IMMUTABLE)", Placeholder: "Needs a caller-set Message ID to work. Cannot be added later"},
	{Name: "duplicate_detection_history_seconds", Type: core.ConnectionTypeInteger, Label: "Duplicate Detection Window (seconds)", Placeholder: "600", Visible: &core.VisibleWhen{Field: "requires_duplicate_detection", Values: []string{"true"}}},
	{Name: "dead_lettering_on_message_expiration", Type: core.ConnectionTypeBoolean, Label: "Dead-Letter On Expiry", Placeholder: "Move expired messages to the dead-letter queue instead of dropping them"},
	{Name: "auto_delete_on_idle_seconds", Type: core.ConnectionTypeInteger, Label: "Auto-Delete On Idle (seconds)", Placeholder: "Blank to never auto-delete; minimum 300 when set"},
	{Name: "max_size_megabytes", Type: core.ConnectionTypeInteger, Label: "Max Size (MB)", Placeholder: "1024"},
	{Name: "enable_partitioning", Type: core.ConnectionTypeBoolean, Label: "Enable Partitioning (IMMUTABLE)"},
	{Name: "forward_to", Type: core.ConnectionTypeString, Label: "Forward To", Placeholder: "Another queue or topic in this namespace — auto-forwarding"},
	{Name: "forward_dead_lettered_messages_to", Type: core.ConnectionTypeString, Label: "Forward Dead-Lettered To", Placeholder: "Another entity in this namespace"},
	{Name: "user_metadata", Type: core.ConnectionTypeString, Label: "Notes", Placeholder: "Free text stored on the queue"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Queue"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Queue"},
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

	props := &admin.QueueProperties{}
	sb.SetISODurationIfSet(&props.LockDuration, "lock_duration_seconds", inputs)
	sb.SetISODurationIfSet(&props.DefaultMessageTimeToLive, "default_message_time_to_live_seconds", inputs)
	sb.SetISODurationIfSet(&props.DuplicateDetectionHistoryTimeWindow, "duplicate_detection_history_seconds", inputs)
	sb.SetISODurationIfSet(&props.AutoDeleteOnIdle, "auto_delete_on_idle_seconds", inputs)
	sb.SetInt32IfSet(&props.MaxDeliveryCount, "max_delivery_count", inputs)
	sb.SetInt32IfSet(&props.MaxSizeInMegabytes, "max_size_megabytes", inputs)
	sb.SetBoolIfSet(&props.RequiresSession, "requires_session", inputs)
	sb.SetBoolIfSet(&props.RequiresDuplicateDetection, "requires_duplicate_detection", inputs)
	sb.SetBoolIfSet(&props.DeadLetteringOnMessageExpiration, "dead_lettering_on_message_expiration", inputs)
	sb.SetBoolIfSet(&props.EnablePartitioning, "enable_partitioning", inputs)
	sb.SetStringIfPresent(&props.ForwardTo, "forward_to", inputs)
	sb.SetStringIfPresent(&props.ForwardDeadLetteredMessagesTo, "forward_dead_lettered_messages_to", inputs)
	sb.SetStringIfPresent(&props.UserMetadata, "user_metadata", inputs)

	client, err := sb.NewAdmin(auth)
	if err != nil {
		return sb.Fail(auth, "Could not open the management client", err), nil
	}
	ctx, cancel := sb.AdminContext(flow)
	defer cancel()

	created, err := client.CreateQueue(ctx, queue, props)
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not create queue %s", queue), err), nil
	}
	return sb.ResourceResult(queue, sb.QueueOutput(queue, created), fmt.Sprintf("Created queue %s", queue)), nil
}

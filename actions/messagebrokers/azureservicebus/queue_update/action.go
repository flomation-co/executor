package messagebrokers_azureservicebus_queue_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Update Queue"
	Description  = "Update a queue's mutable properties. Read-modify-write: the queue is fetched first and only the fields set here are changed, because the management API takes a whole QueueProperties and would otherwise reset everything left blank to its default. The immutable flags (session, duplicate detection, partitioning) are not offered — they cannot change."
	Website      = "https://www.flomation.co"
	Icon         = "azure+pen"
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
	{Name: "lock_duration_seconds", Type: core.ConnectionTypeInteger, Label: "Lock Duration (seconds)", Placeholder: "Blank to leave alone; capped at 300 by Service Bus"},
	{Name: "max_delivery_count", Type: core.ConnectionTypeInteger, Label: "Max Delivery Count", Placeholder: "Blank to leave alone"},
	{Name: "default_message_time_to_live_seconds", Type: core.ConnectionTypeInteger, Label: "Default Message TTL (seconds)", Placeholder: "Blank to leave alone"},
	{Name: "auto_delete_on_idle_seconds", Type: core.ConnectionTypeInteger, Label: "Auto-Delete On Idle (seconds)", Placeholder: "Blank to leave alone"},
	{Name: "max_size_megabytes", Type: core.ConnectionTypeInteger, Label: "Max Size (MB)", Placeholder: "Blank to leave alone"},
	{Name: "dead_lettering_on_message_expiration", Type: core.ConnectionTypeBoolean, Label: "Dead-Letter On Expiry", Placeholder: "Untouched leaves it alone"},
	{Name: "forward_to", Type: core.ConnectionTypeString, Label: "Forward To", Placeholder: "Blank to leave alone"},
	{Name: "forward_dead_lettered_messages_to", Type: core.ConnectionTypeString, Label: "Forward Dead-Lettered To", Placeholder: "Blank to leave alone"},
	{Name: "user_metadata", Type: core.ConnectionTypeString, Label: "Notes", Placeholder: "Blank to leave alone"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{{Name: "Leave unchanged", Value: ""}, {Name: "Active", Value: "Active"}, {Name: "Disabled (stops sends and receives)", Value: "Disabled"}}},
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
	client, err := sb.NewAdmin(auth)
	if err != nil {
		return sb.Fail(auth, "Could not open the management client", err), nil
	}
	ctx, cancel := sb.AdminContext(flow)
	defer cancel()

	// UpdateQueue replaces the entity's properties wholesale, so the current
	// ones are the starting point. Sending a partially-filled struct would
	// silently reset every property the operator left blank.
	props, err := client.GetQueue(ctx, queue)
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not read queue %s before updating it", queue), err), nil
	}

	sb.SetISODurationIfSet(&props.LockDuration, "lock_duration_seconds", inputs)
	sb.SetISODurationIfSet(&props.DefaultMessageTimeToLive, "default_message_time_to_live_seconds", inputs)
	sb.SetISODurationIfSet(&props.AutoDeleteOnIdle, "auto_delete_on_idle_seconds", inputs)
	sb.SetInt32IfSet(&props.MaxDeliveryCount, "max_delivery_count", inputs)
	sb.SetInt32IfSet(&props.MaxSizeInMegabytes, "max_size_megabytes", inputs)
	sb.SetBoolIfSet(&props.DeadLetteringOnMessageExpiration, "dead_lettering_on_message_expiration", inputs)
	sb.SetStringIfPresent(&props.ForwardTo, "forward_to", inputs)
	sb.SetStringIfPresent(&props.ForwardDeadLetteredMessagesTo, "forward_dead_lettered_messages_to", inputs)
	sb.SetStringIfPresent(&props.UserMetadata, "user_metadata", inputs)
	if v := sb.OptionalString("status", inputs); v != "" {
		status := admin.EntityStatus(v)
		props.Status = &status
	}

	updated, err := client.UpdateQueue(ctx, queue, props)
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not update queue %s", queue), err), nil
	}
	return sb.ResourceResult(queue, sb.QueueOutput(queue, updated), fmt.Sprintf("Updated queue %s", queue)), nil
}

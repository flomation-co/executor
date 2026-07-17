package messagebrokers_azureservicebus_topic_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Create Topic"
	Description  = "Create a topic. A topic on its own is a black hole: messages sent to a topic with no subscriptions are discarded and the send still reports success. Create at least one subscription before sending."
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
	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic", Placeholder: "order-events", Required: true},
	{Name: "default_message_time_to_live_seconds", Type: core.ConnectionTypeInteger, Label: "Default Message TTL (seconds)", Placeholder: "Blank for the maximum"},
	{Name: "max_size_megabytes", Type: core.ConnectionTypeInteger, Label: "Max Size (MB)", Placeholder: "1024"},
	{Name: "requires_duplicate_detection", Type: core.ConnectionTypeBoolean, Label: "Requires Duplicate Detection (IMMUTABLE)", Placeholder: "Cannot be added after creation"},
	{Name: "duplicate_detection_history_seconds", Type: core.ConnectionTypeInteger, Label: "Duplicate Detection Window (seconds)", Placeholder: "600", Visible: &core.VisibleWhen{Field: "requires_duplicate_detection", Values: []string{"true"}}},
	{Name: "support_ordering", Type: core.ConnectionTypeBoolean, Label: "Support Ordering", Placeholder: "Forward to subscriptions in order"},
	{Name: "enable_partitioning", Type: core.ConnectionTypeBoolean, Label: "Enable Partitioning (IMMUTABLE)"},
	{Name: "auto_delete_on_idle_seconds", Type: core.ConnectionTypeInteger, Label: "Auto-Delete On Idle (seconds)", Placeholder: "Blank to never auto-delete; minimum 300 when set"},
	{Name: "user_metadata", Type: core.ConnectionTypeString, Label: "Notes", Placeholder: "Free text stored on the topic"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Topic"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Topic"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sb.GetAuth(inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	topic, err := sb.RequiredString("topic", inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}

	props := &admin.TopicProperties{}
	sb.SetISODurationIfSet(&props.DefaultMessageTimeToLive, "default_message_time_to_live_seconds", inputs)
	sb.SetISODurationIfSet(&props.DuplicateDetectionHistoryTimeWindow, "duplicate_detection_history_seconds", inputs)
	sb.SetISODurationIfSet(&props.AutoDeleteOnIdle, "auto_delete_on_idle_seconds", inputs)
	sb.SetInt32IfSet(&props.MaxSizeInMegabytes, "max_size_megabytes", inputs)
	sb.SetBoolIfSet(&props.RequiresDuplicateDetection, "requires_duplicate_detection", inputs)
	sb.SetBoolIfSet(&props.SupportOrdering, "support_ordering", inputs)
	sb.SetBoolIfSet(&props.EnablePartitioning, "enable_partitioning", inputs)
	sb.SetStringIfPresent(&props.UserMetadata, "user_metadata", inputs)

	client, err := sb.NewAdmin(auth)
	if err != nil {
		return sb.Fail(auth, "Could not open the management client", err), nil
	}
	ctx, cancel := sb.AdminContext(flow)
	defer cancel()

	created, err := client.CreateTopic(ctx, topic, props)
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not create topic %s", topic), err), nil
	}
	return sb.ResourceResult(topic, sb.TopicOutput(topic, created), fmt.Sprintf("Created topic %s — add a subscription before sending to it, or its messages go nowhere", topic)), nil
}

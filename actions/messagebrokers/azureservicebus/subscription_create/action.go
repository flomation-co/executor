package messagebrokers_azureservicebus_subscription_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Create Subscription"
	Description  = "Create a subscription on a topic. Every new subscription ships with a $Default rule that matches everything — add a filter with Create Rule and then DELETE $Default, or the filter will appear to do nothing. Requires Session is immutable here too."
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
	{Name: "subscription", Type: core.ConnectionTypeString, Label: "Subscription", Placeholder: "billing — messages live on the subscription, never on the topic", Required: true},
	{Name: "lock_duration_seconds", Type: core.ConnectionTypeInteger, Label: "Lock Duration (seconds)", Placeholder: "60 — capped at 300 by Service Bus"},
	{Name: "max_delivery_count", Type: core.ConnectionTypeInteger, Label: "Max Delivery Count", Placeholder: "10 — attempts before a message is dead-lettered automatically"},
	{Name: "default_message_time_to_live_seconds", Type: core.ConnectionTypeInteger, Label: "Default Message TTL (seconds)", Placeholder: "Blank for the topic default"},
	{Name: "requires_session", Type: core.ConnectionTypeBoolean, Label: "Requires Session (IMMUTABLE)", Placeholder: "Cannot be changed after creation"},
	{Name: "dead_lettering_on_message_expiration", Type: core.ConnectionTypeBoolean, Label: "Dead-Letter On Expiry"},
	{Name: "dead_lettering_on_filter_evaluation_exceptions", Type: core.ConnectionTypeBoolean, Label: "Dead-Letter On Filter Errors", Placeholder: "Dead-letter a message whose rule evaluation threw, rather than dropping it"},
	{Name: "auto_delete_on_idle_seconds", Type: core.ConnectionTypeInteger, Label: "Auto-Delete On Idle (seconds)", Placeholder: "Blank to never auto-delete; minimum 300 when set"},
	{Name: "forward_to", Type: core.ConnectionTypeString, Label: "Forward To", Placeholder: "Another queue or topic in this namespace"},
	{Name: "forward_dead_lettered_messages_to", Type: core.ConnectionTypeString, Label: "Forward Dead-Lettered To", Placeholder: "Another entity in this namespace"},
	{Name: "user_metadata", Type: core.ConnectionTypeString, Label: "Notes", Placeholder: "Free text stored on the subscription"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Subscription"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Subscription"},
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
	subscription, err := sb.RequiredString("subscription", inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}

	props := &admin.SubscriptionProperties{}
	sb.SetISODurationIfSet(&props.LockDuration, "lock_duration_seconds", inputs)
	sb.SetISODurationIfSet(&props.DefaultMessageTimeToLive, "default_message_time_to_live_seconds", inputs)
	sb.SetISODurationIfSet(&props.AutoDeleteOnIdle, "auto_delete_on_idle_seconds", inputs)
	sb.SetInt32IfSet(&props.MaxDeliveryCount, "max_delivery_count", inputs)
	sb.SetBoolIfSet(&props.RequiresSession, "requires_session", inputs)
	sb.SetBoolIfSet(&props.DeadLetteringOnMessageExpiration, "dead_lettering_on_message_expiration", inputs)
	sb.SetBoolIfSet(&props.EnableDeadLetteringOnFilterEvaluationExceptions, "dead_lettering_on_filter_evaluation_exceptions", inputs)
	sb.SetStringIfPresent(&props.ForwardTo, "forward_to", inputs)
	sb.SetStringIfPresent(&props.ForwardDeadLetteredMessagesTo, "forward_dead_lettered_messages_to", inputs)
	sb.SetStringIfPresent(&props.UserMetadata, "user_metadata", inputs)

	client, err := sb.NewAdmin(auth)
	if err != nil {
		return sb.Fail(auth, "Could not open the management client", err), nil
	}
	ctx, cancel := sb.AdminContext(flow)
	defer cancel()

	created, err := client.CreateSubscription(ctx, topic, subscription, props)
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not create subscription %s on topic %s", subscription, topic), err), nil
	}
	return sb.ResourceResult(subscription, sb.SubscriptionOutput(topic, subscription, created),
		fmt.Sprintf("Created subscription %s on topic %s — it starts with a $Default rule that matches every message", subscription, topic)), nil
}

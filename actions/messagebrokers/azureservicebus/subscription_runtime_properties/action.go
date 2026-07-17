package messagebrokers_azureservicebus_subscription_runtime_properties

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Get Subscription Runtime Properties"
	Description  = "Get a subscription's live counts: active, dead-lettered and transfer message counts. The topic-side twin of Get Queue Runtime Properties, and the read a monitoring flow is built on."
	Website      = "https://www.flomation.co"
	Icon         = "azure+gauge"
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
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Subscription"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Runtime Properties"},
	{Name: "active_message_count", Type: core.ConnectionTypeInteger, Label: "Active Messages"},
	{Name: "dead_letter_message_count", Type: core.ConnectionTypeInteger, Label: "Dead-Lettered Messages"},
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
	client, err := sb.NewAdmin(auth)
	if err != nil {
		return sb.Fail(auth, "Could not open the management client", err), nil
	}
	ctx, cancel := sb.AdminContext(flow)
	defer cancel()

	rt, err := client.GetSubscriptionRuntimeProperties(ctx, topic, subscription)
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not get runtime properties for subscription %s on topic %s", subscription, topic), err), nil
	}
	result := map[string]interface{}{
		"name":                               subscription,
		"topic":                              topic,
		"total_message_count":                rt.TotalMessageCount,
		"active_message_count":               int(rt.ActiveMessageCount),
		"dead_letter_message_count":          int(rt.DeadLetterMessageCount),
		"transfer_message_count":             int(rt.TransferMessageCount),
		"transfer_dead_letter_message_count": int(rt.TransferDeadLetterMessageCount),
		"created_at":                         rt.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":                         rt.UpdatedAt.UTC().Format(time.RFC3339),
		"accessed_at":                        rt.AccessedAt.UTC().Format(time.RFC3339),
	}

	out := sb.ResourceResult(subscription, result, fmt.Sprintf("Subscription %s on topic %s: %d active, %d dead-lettered",
		subscription, topic, rt.ActiveMessageCount, rt.DeadLetterMessageCount))
	out["active_message_count"] = int(rt.ActiveMessageCount)
	out["dead_letter_message_count"] = int(rt.DeadLetterMessageCount)
	return out, nil
}

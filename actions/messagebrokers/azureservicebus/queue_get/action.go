package messagebrokers_azureservicebus_queue_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Get Queue"
	Description  = "Get a queue's configuration: lock duration, max delivery count, TTL, the session and duplicate-detection flags, and the size cap. For live message counts use Get Queue Runtime Properties instead — configuration and counts are different calls."
	Website      = "https://www.flomation.co"
	Icon         = "azure+magnifying-glass"
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

	props, err := client.GetQueue(ctx, queue)
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not get queue %s", queue), err), nil
	}
	return sb.ResourceResult(queue, sb.QueueOutput(queue, props), fmt.Sprintf("Fetched queue %s", queue)), nil
}

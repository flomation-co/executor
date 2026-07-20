package messagebrokers_azureservicebus_queue_list

import (
	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Get Many Queues"
	Description  = "List the queues in the namespace with their configuration. Needs a namespace-level connection string (or an Entra principal): an entity-scoped one cannot reach the management API at all. The management plane is rate-limited far harder than the data plane — do not call this in a tight loop."
	Website      = "https://www.flomation.co"
	Icon         = "azure+list"
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
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Queues"},
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
	client, err := sb.NewAdmin(auth)
	if err != nil {
		return sb.Fail(auth, "Could not open the management client", err), nil
	}
	ctx, cancel := sb.AdminContext(flow)
	defer cancel()

	items, err := client.ListQueues(ctx, sb.ClampLimit(inputs))
	if err != nil {
		return sb.Fail(auth, "Could not list queues", err), nil
	}
	out := make([]interface{}, 0, len(items))
	for _, q := range items {
		out = append(out, sb.QueueOutput(q.QueueName, q.QueueProperties))
	}
	return sb.ListResult(out, sb.ListSummary("queue", len(out))), nil
}

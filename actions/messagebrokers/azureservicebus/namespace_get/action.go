package messagebrokers_azureservicebus_namespace_get

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Get Namespace"
	Description  = "Get the namespace's properties: name, SKU tier and messaging units. The natural test-connection probe — it proves the credentials and reachability without touching a queue, and the tier tells you the message size cap you are working under (256KB on Standard, 100MB on Premium)."
	Website      = "https://www.flomation.co"
	Icon         = "azure+circle-nodes"
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
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Namespace"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Namespace"},
	{Name: "sku", Type: core.ConnectionTypeString, Label: "SKU"},
	{Name: "max_message_bytes", Type: core.ConnectionTypeInteger, Label: "Max Message Size (bytes)"},
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

	props, err := client.GetNamespaceProperties(ctx)
	if err != nil {
		return sb.Fail(auth, "Could not get the namespace properties", err), nil
	}

	// The size cap is tier-dependent, and an operator who has read the general
	// Service Bus literature will assume 256KB. Saying which one applies here
	// is the whole reason this action is worth having beyond a ping.
	maxBytes := sb.StandardMaxMessageBytes
	if strings.EqualFold(props.SKU, "Premium") {
		maxBytes = 100 * 1024 * 1024
	}

	result := map[string]interface{}{
		"name":              props.Name,
		"sku":               props.SKU,
		"created_time":      props.CreatedTime.UTC().Format(time.RFC3339),
		"modified_time":     props.ModifiedTime.UTC().Format(time.RFC3339),
		"max_message_bytes": maxBytes,
	}
	if props.MessagingUnits != nil {
		result["messaging_units"] = *props.MessagingUnits
	}

	out := sb.ResourceResult(props.Name, result, fmt.Sprintf("Namespace %s is on the %s tier (messages up to %d bytes, headers and properties included)", props.Name, props.SKU, maxBytes))
	out["sku"] = props.SKU
	out["max_message_bytes"] = maxBytes
	return out, nil
}

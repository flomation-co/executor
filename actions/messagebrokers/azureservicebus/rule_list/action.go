package messagebrokers_azureservicebus_rule_list

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Get Many Rules"
	Description  = "List a subscription's filter rules. Worth running whenever a filter seems to be ignored: every subscription is created with a $Default rule that matches everything, and while it exists any filter you add is redundant."
	Website      = "https://www.flomation.co"
	Icon         = "azure+filter"
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
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Rules"},
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

	rules, err := client.ListRules(ctx, topic, subscription, sb.ClampLimit(inputs))
	if err != nil {
		return sb.Fail(auth, "Could not list rules on subscription "+subscription, err), nil
	}
	// A subscription that does not exist lists as ZERO rules rather than as an
	// error. That is especially misleading here: EVERY real subscription has at
	// least the $Default rule, so zero rules is not a state a live
	// subscription can be in — it means the subscription is not there.
	if err := sb.CheckListParent(ctx, len(rules),
		func(c context.Context) (bool, error) { return client.SubscriptionExists(c, topic, subscription) },
		fmt.Sprintf("no subscription named %q exists on topic %q — every existing subscription has at least the $Default rule, so an empty rule list means the subscription itself is missing", subscription, topic)); err != nil {
		return sb.ErrorResult("Could not list rules on subscription " + subscription + ": " + err.Error()), nil
	}
	out := make([]interface{}, 0, len(rules))
	for _, r := range rules {
		out = append(out, sb.RuleOutput(r))
	}
	return sb.ListResult(out, sb.ListSummary("rule", len(out))), nil
}

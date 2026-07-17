package messagebrokers_azureservicebus_rule_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Service Bus: Create Rule"
	Description  = "Create a filter rule on a subscription — a SQL filter, a correlation filter, or a true/false filter, with an optional SQL action that rewrites matched messages. Filter rules are what make a topic worth using instead of N queues. Remember to delete the subscription's $Default rule afterwards: it matches everything, so while it exists the new filter narrows nothing."
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
	{Name: "rule_name", Type: core.ConnectionTypeString, Label: "Rule Name", Placeholder: "high-value-orders", Required: true},
	{Name: "filter_type", Type: core.ConnectionTypeString, Label: "Filter Type", Options: []core.ConnectionOption{{Name: "SQL expression", Value: "sql"}, {Name: "Correlation (property equality — cheaper)", Value: "correlation"}, {Name: "True (match everything)", Value: "true"}, {Name: "False (match nothing)", Value: "false"}}},
	{Name: "sql_expression", Type: core.ConnectionTypeString, Label: "SQL Filter", Placeholder: `total > 100 AND region = 'UK' — matches on application properties and broker properties`, Visible: &core.VisibleWhen{Field: "filter_type", Values: []string{"", "sql"}}},
	{Name: "correlation_filter", Type: core.ConnectionTypeObject, Label: "Correlation Filter (JSON)", Placeholder: `{"subject":"order.created","application_properties":{"region":"UK"}} — every named field must match exactly`, Visible: &core.VisibleWhen{Field: "filter_type", Values: []string{"correlation"}}},
	{Name: "action_sql_expression", Type: core.ConnectionTypeString, Label: "SQL Action", Placeholder: `SET priority = 'high' — optional; rewrites the message as it enters the subscription`, Visible: &core.VisibleWhen{Field: "filter_type", Values: []string{"", "sql", "correlation"}}},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Rule"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Rule"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// buildFilter maps the filter_type switch onto the SDK's filter types. A
// correlation filter is a set of equality tests on named properties; the
// broker evaluates it far more cheaply than a SQL filter, which is why it is
// worth exposing separately rather than telling operators to write SQL.
func buildFilter(inputs []*core.Connection) (admin.RuleFilter, error) {
	switch sb.OptionalString("filter_type", inputs) {
	case "", "sql":
		expr, err := sb.RequiredString("sql_expression", inputs)
		if err != nil {
			return nil, fmt.Errorf("sql_expression is required for a SQL filter, e.g. total > 100")
		}
		return &admin.SQLFilter{Expression: expr}, nil
	case "correlation":
		raw, err := sb.OptionalPropertyMap("correlation_filter", inputs)
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			return nil, fmt.Errorf(`correlation_filter is required for a correlation filter, e.g. {"subject":"order.created"}`)
		}
		return correlationFilter(raw)
	case "true":
		return &admin.TrueFilter{}, nil
	case "false":
		return &admin.FalseFilter{}, nil
	default:
		return nil, fmt.Errorf("filter_type must be sql, correlation, true or false")
	}
}

// correlationFilter translates the operator-facing JSON into the SDK's struct.
// The field names mirror the message outputs so the same names mean the same
// thing on both sides of a flow.
func correlationFilter(raw map[string]interface{}) (*admin.CorrelationFilter, error) {
	f := &admin.CorrelationFilter{}
	targets := map[string]**string{
		"correlation_id":      &f.CorrelationID,
		"message_id":          &f.MessageID,
		"subject":             &f.Subject,
		"reply_to":            &f.ReplyTo,
		"reply_to_session_id": &f.ReplyToSessionID,
		"session_id":          &f.SessionID,
		"content_type":        &f.ContentType,
		"to":                  &f.To,
	}
	for key, value := range raw {
		if key == "application_properties" {
			props, ok := value.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("correlation_filter.application_properties must be a JSON object")
			}
			f.ApplicationProperties = props
			continue
		}
		dst, known := targets[key]
		if !known {
			return nil, fmt.Errorf("correlation_filter has no field %q — allowed: correlation_id, message_id, subject, reply_to, reply_to_session_id, session_id, content_type, to, application_properties", key)
		}
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("correlation_filter.%s must be a string", key)
		}
		*dst = &s
	}
	return f, nil
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
	ruleName, err := sb.RequiredString("rule_name", inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}
	filter, err := buildFilter(inputs)
	if err != nil {
		return sb.ErrorResult(err.Error()), nil
	}

	opts := &admin.CreateRuleOptions{Name: &ruleName, Filter: filter}
	if expr := sb.OptionalString("action_sql_expression", inputs); expr != "" {
		opts.Action = &admin.SQLAction{Expression: expr}
	}

	client, err := sb.NewAdmin(auth)
	if err != nil {
		return sb.Fail(auth, "Could not open the management client", err), nil
	}
	ctx, cancel := sb.AdminContext(flow)
	defer cancel()

	created, err := client.CreateRule(ctx, topic, subscription, opts)
	if err != nil {
		return sb.Fail(auth, fmt.Sprintf("Could not create rule %s on subscription %s", ruleName, subscription), err), nil
	}
	return sb.ResourceResult(ruleName, sb.RuleOutput(created),
		fmt.Sprintf("Created rule %s on subscription %s — delete the $Default rule if this filter should actually narrow anything", ruleName, subscription)), nil
}

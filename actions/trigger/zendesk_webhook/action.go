package zendesk_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk Webhook Trigger"
	Description  = "Triggers a flow when a Zendesk ticket is created or updated. Flomation registers the Zendesk webhook and business rule automatically. Leave Conditions empty to fire on every ticket create/update, or supply Zendesk trigger conditions as JSON to narrow it."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk"
	Date         = "03/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token to register the webhook)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token (used to register the webhook)"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
	{Name: "conditions", Type: core.ConnectionTypeObject, Label: "Conditions (JSON)", Placeholder: `Optional — Zendesk trigger conditions, e.g. {"all":[{"field":"priority","operator":"is","value":"high"}]}`},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "ticket_id", Type: core.ConnectionTypeString, Label: "Ticket ID"},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "priority", Type: core.ConnectionTypeString, Label: "Priority"},
	{Name: "requester_email", Type: core.ConnectionTypeString, Label: "Requester Email"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Latest Description"},
	{Name: "via", Type: core.ConnectionTypeString, Label: "Channel"},
	{Name: "payload", Type: core.ConnectionTypeObject, Label: "Payload"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they carry the credentials or the internal registration settings.
var configInputs = map[string]bool{
	"subdomain":   true,
	"email":       true,
	"api_token":   true,
	"oauth_token": true,
	"conditions":  true,
	"__node_id":   true,
}

// Execute runs at flow execution time, with the webhook payload injected by
// launch (which registered the Zendesk webhook + business rule, verified the
// signature, and parsed the body). It copies the injected fields to the result
// and derives a human-readable content summary — mirroring the Calendly trigger.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Zendesk webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	result["content"] = buildContentSummary(result)

	return result, nil
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func buildContentSummary(data map[string]interface{}) string {
	id := str(data["ticket_id"])
	subject := str(data["subject"])
	status := str(data["status"])

	switch {
	case id != "" && subject != "":
		return fmt.Sprintf("[Zendesk] Ticket #%s — %s (%s)", id, subject, status)
	case id != "":
		return fmt.Sprintf("[Zendesk] Ticket #%s (%s)", id, status)
	default:
		return "[Zendesk] Ticket event"
	}
}

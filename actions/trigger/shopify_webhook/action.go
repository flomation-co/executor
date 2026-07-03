package shopify_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Shopify Webhook Trigger"
	Description  = "Triggers a flow when a Shopify webhook event is received (orders, products, ...). Verify with your app's API secret key."
	Website      = "https://www.flomation.co"
	Icon         = "shopify"
	Date         = "02/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret Key", Placeholder: "Shopify app API secret key (for X-Shopify-Hmac-Sha256 validation)", Required: true},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Topic Filter", Placeholder: "Comma-separated topics: orders/create,orders/updated,products/update"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic"},
	{Name: "shop_domain", Type: core.ConnectionTypeString, Label: "Shop Domain"},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version"},
	{Name: "webhook_id", Type: core.ConnectionTypeString, Label: "Webhook ID"},
	{Name: "resource_id", Type: core.ConnectionTypeString, Label: "Resource ID"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they contain the signing secret or internal filter settings.
var configInputs = map[string]bool{
	"app_secret":   true,
	"event_filter": true,
	"__node_id":    true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Shopify webhook trigger")

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
	topic := str(data["topic"])
	shop := str(data["shop_domain"])
	resource := str(data["resource_id"])

	if topic == "" {
		topic = "event"
	}
	if resource != "" {
		return fmt.Sprintf("[Shopify] %s (%s) on %s", topic, resource, shop)
	}
	return fmt.Sprintf("[Shopify] %s on %s", topic, shop)
}

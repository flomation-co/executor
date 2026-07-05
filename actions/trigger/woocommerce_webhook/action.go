package woocommerce_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WooCommerce Webhook Trigger"
	Description  = "Triggers a flow when a WooCommerce event occurs (order/product/customer/coupon created, updated or deleted). The webhook is registered and signature-verified automatically."
	Website      = "https://www.flomation.co"
	Icon         = "woocommerce"
	Date         = "05/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Store URL", Placeholder: "https://your-store.com — your store's root URL, not the /wp-json path", Required: true},
	{Name: "consumer_key", Type: core.ConnectionTypeSecret, Label: "Consumer Key", Placeholder: "ck_... — used to register the webhook", Required: true},
	{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Label: "Consumer Secret", Placeholder: "cs_...", Required: true},
	{Name: "credentials_in_query", Type: core.ConnectionTypeBoolean, Label: "Send Credentials in Query String", Placeholder: "Enable only if you see a \"Consumer key is missing\" error"},
	{Name: "events", Type: core.ConnectionTypeMultiSelect, Label: "Events", Required: true, Options: []core.ConnectionOption{
		{Name: "Order Created", Value: "order.created"},
		{Name: "Order Updated", Value: "order.updated"},
		{Name: "Order Deleted", Value: "order.deleted"},
		{Name: "Product Created", Value: "product.created"},
		{Name: "Product Updated", Value: "product.updated"},
		{Name: "Product Deleted", Value: "product.deleted"},
		{Name: "Customer Created", Value: "customer.created"},
		{Name: "Customer Updated", Value: "customer.updated"},
		{Name: "Customer Deleted", Value: "customer.deleted"},
		{Name: "Coupon Created", Value: "coupon.created"},
		{Name: "Coupon Updated", Value: "coupon.updated"},
		{Name: "Coupon Deleted", Value: "coupon.deleted"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic"},
	{Name: "resource", Type: core.ConnectionTypeString, Label: "Resource"},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event"},
	{Name: "resource_id", Type: core.ConnectionTypeString, Label: "Resource ID"},
	{Name: "webhook_id", Type: core.ConnectionTypeString, Label: "Webhook ID"},
	{Name: "delivery_id", Type: core.ConnectionTypeString, Label: "Delivery ID"},
	{Name: "source", Type: core.ConnectionTypeString, Label: "Source Store"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they carry the API key pair or internal registration settings.
var configInputs = map[string]bool{
	"url":                  true,
	"consumer_key":         true,
	"consumer_secret":      true,
	"credentials_in_query": true,
	"events":               true,
	"__node_id":            true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing WooCommerce webhook trigger")

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
	source := str(data["source"])
	resourceID := str(data["resource_id"])

	if topic == "" {
		topic = "event"
	}
	if resourceID != "" {
		return fmt.Sprintf("[WooCommerce] %s (%s) on %s", topic, resourceID, source)
	}
	return fmt.Sprintf("[WooCommerce] %s on %s", topic, source)
}

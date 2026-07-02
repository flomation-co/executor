package shopify_webhook

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestExecuteEchoesEventFieldsAndStripsConfig(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		// Config fields — must NOT appear in the output.
		{Name: "app_secret", Type: core.ConnectionTypeSecret, Value: "shpss_secret"},
		{Name: "event_filter", Type: core.ConnectionTypeString, Value: "orders/create"},
		{Name: "__node_id", Type: core.ConnectionTypeString, Value: "node-1"},
		// Event fields injected by Launch — must be echoed.
		{Name: "topic", Type: core.ConnectionTypeString, Value: "orders/create"},
		{Name: "shop_domain", Type: core.ConnectionTypeString, Value: "flomation-dev.myshopify.com"},
		{Name: "api_version", Type: core.ConnectionTypeString, Value: "2025-01"},
		{Name: "webhook_id", Type: core.ConnectionTypeString, Value: "wh-123"},
		{Name: "resource_id", Type: core.ConnectionTypeString, Value: "450789469"},
		{Name: "body", Type: core.ConnectionTypeString, Value: `{"id":450789469}`},
	}

	out, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())

	// Config inputs are stripped.
	Expect(out).To(Not(HaveKey("app_secret")))
	Expect(out).To(Not(HaveKey("event_filter")))
	Expect(out).To(Not(HaveKey("__node_id")))

	// Event fields flow through.
	Expect(out["topic"]).To(Equal("orders/create"))
	Expect(out["shop_domain"]).To(Equal("flomation-dev.myshopify.com"))
	Expect(out["resource_id"]).To(Equal("450789469"))
	Expect(out["body"]).To(Equal(`{"id":450789469}`))

	// Content summary is synthesised.
	Expect(out["content"]).To(Equal("[Shopify] orders/create (450789469) on flomation-dev.myshopify.com"))
}

func TestContentSummaryWithoutResourceID(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "topic", Type: core.ConnectionTypeString, Value: "app/uninstalled"},
		{Name: "shop_domain", Type: core.ConnectionTypeString, Value: "flomation-dev.myshopify.com"},
	})
	Expect(err).To(BeNil())
	Expect(out["content"]).To(Equal("[Shopify] app/uninstalled on flomation-dev.myshopify.com"))
}

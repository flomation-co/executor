package woocommerce_webhook

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestExecuteEchoesEventFieldsAndStripsConfig(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		// Config fields — must NOT appear in the output.
		{Name: "url", Type: core.ConnectionTypeString, Value: "https://store.example.com"},
		{Name: "consumer_key", Type: core.ConnectionTypeSecret, Value: "ck_x"},
		{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Value: "cs_y"},
		{Name: "credentials_in_query", Type: core.ConnectionTypeBoolean, Value: false},
		{Name: "events", Type: core.ConnectionTypeMultiSelect, Value: `["order.created"]`},
		{Name: "__node_id", Type: core.ConnectionTypeString, Value: "node-1"},
		// Event fields injected by Launch — must be echoed.
		{Name: "topic", Type: core.ConnectionTypeString, Value: "order.created"},
		{Name: "resource", Type: core.ConnectionTypeString, Value: "order"},
		{Name: "event", Type: core.ConnectionTypeString, Value: "created"},
		{Name: "resource_id", Type: core.ConnectionTypeString, Value: "725"},
		{Name: "webhook_id", Type: core.ConnectionTypeString, Value: "12"},
		{Name: "source", Type: core.ConnectionTypeString, Value: "https://store.example.com"},
		{Name: "body", Type: core.ConnectionTypeString, Value: `{"id":725}`},
	}

	out, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())

	// Config inputs are stripped.
	for _, k := range []string{"url", "consumer_key", "consumer_secret", "credentials_in_query", "events", "__node_id"} {
		Expect(out).To(Not(HaveKey(k)), "config key leaked: %s", k)
	}

	// Event fields flow through.
	Expect(out["topic"]).To(Equal("order.created"))
	Expect(out["resource"]).To(Equal("order"))
	Expect(out["resource_id"]).To(Equal("725"))
	Expect(out["body"]).To(Equal(`{"id":725}`))

	// Content summary is synthesised.
	Expect(out["content"]).To(Equal("[WooCommerce] order.created (725) on https://store.example.com"))
}

func TestContentSummaryWithoutResourceID(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "topic", Type: core.ConnectionTypeString, Value: "product.deleted"},
		{Name: "source", Type: core.ConnectionTypeString, Value: "https://store.example.com"},
	})
	Expect(err).To(BeNil())
	Expect(out["content"]).To(Equal("[WooCommerce] product.deleted on https://store.example.com"))
}

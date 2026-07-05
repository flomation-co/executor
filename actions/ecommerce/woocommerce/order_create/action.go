package ecommerce_woocommerce_order_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	woocommerce "flomation.app/automate/executor/actions/ecommerce/woocommerce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WooCommerce: Create Order"
	Description  = "Create an order in your WooCommerce store. Provide line items as JSON; set common fields directly or add any other order field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "woocommerce+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Store URL", Placeholder: "https://your-store.com", Required: true},
	{Name: "consumer_key", Type: core.ConnectionTypeSecret, Label: "Consumer Key", Placeholder: "ck_...", Required: true},
	{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Label: "Consumer Secret", Placeholder: "cs_...", Required: true},
	{Name: "credentials_in_query", Type: core.ConnectionTypeBoolean, Label: "Send Credentials in Query String", Placeholder: "Enable only if you see a \"Consumer key is missing\" error"},
	{
		Name:  "status",
		Type:  core.ConnectionTypeString,
		Label: "Status",
		Options: []core.ConnectionOption{
			{Name: "Pending Payment", Value: "pending"},
			{Name: "Processing", Value: "processing"},
			{Name: "On Hold", Value: "on-hold"},
			{Name: "Completed", Value: "completed"},
			{Name: "Cancelled", Value: "cancelled"},
			{Name: "Refunded", Value: "refunded"},
			{Name: "Failed", Value: "failed"},
		},
	},
	{Name: "currency", Type: core.ConnectionTypeString, Label: "Currency", Placeholder: "GBP, USD, EUR (ISO 4217)"},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID", Placeholder: "0 for a guest order"},
	{Name: "customer_note", Type: core.ConnectionTypeText, Label: "Customer Note"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Parent Order ID"},
	{Name: "payment_method", Type: core.ConnectionTypeString, Label: "Payment Method ID", Placeholder: "bacs, cheque, cod, paypal..."},
	{Name: "payment_method_title", Type: core.ConnectionTypeString, Label: "Payment Method Title", Placeholder: "Direct Bank Transfer"},
	{Name: "transaction_id", Type: core.ConnectionTypeString, Label: "Transaction ID"},
	{Name: "set_paid", Type: core.ConnectionTypeBoolean, Label: "Set Paid", Placeholder: "Mark the order as paid and reduce stock"},
	{Name: "line_items", Type: core.ConnectionTypeObject, Label: "Line Items (JSON)", Placeholder: `[{"product_id":93,"quantity":2}]`},
	{Name: "billing", Type: core.ConnectionTypeObject, Label: "Billing Address (JSON)", Placeholder: `{"first_name":"Jane","last_name":"Doe","address_1":"1 Main St","city":"London","postcode":"SW1","country":"GB","email":"jane@example.com"}`},
	{Name: "shipping", Type: core.ConnectionTypeObject, Label: "Shipping Address (JSON)", Placeholder: `{"first_name":"Jane","last_name":"Doe","address_1":"1 Main St","city":"London","postcode":"SW1","country":"GB"}`},
	{Name: "shipping_lines", Type: core.ConnectionTypeObject, Label: "Shipping Lines (JSON)", Placeholder: `[{"method_id":"flat_rate","method_title":"Flat Rate","total":"5.00"}]`},
	{Name: "fee_lines", Type: core.ConnectionTypeObject, Label: "Fee Lines (JSON)", Placeholder: `[{"name":"Gift Wrap","total":"2.50"}]`},
	{Name: "coupon_lines", Type: core.ConnectionTypeObject, Label: "Coupon Lines (JSON)", Placeholder: `[{"code":"SAVE10"}]`},
	{Name: "meta_data", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `[{"key":"source","value":"web"}]`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"created_via":"api"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Order ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Order"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := woocommerce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	order := map[string]interface{}{}
	woocommerce.SetIfPresent(order, inputs, "status", "status")
	woocommerce.SetIfPresent(order, inputs, "currency", "currency")
	woocommerce.SetIfPresent(order, inputs, "customer_note", "customer_note")
	woocommerce.SetIfPresent(order, inputs, "payment_method", "payment_method")
	woocommerce.SetIfPresent(order, inputs, "payment_method_title", "payment_method_title")
	woocommerce.SetIfPresent(order, inputs, "transaction_id", "transaction_id")
	woocommerce.SetBoolIfSet(order, inputs, "set_paid", "set_paid")
	for field, input := range map[string]string{"customer_id": "customer_id", "parent_id": "parent_id"} {
		if err := woocommerce.SetIntIfPresent(order, inputs, field, input); err != nil {
			return woocommerce.ErrorResult(err.Error()), nil
		}
	}
	for field, input := range map[string]string{
		"line_items":     "line_items",
		"billing":        "billing",
		"shipping":       "shipping",
		"shipping_lines": "shipping_lines",
		"fee_lines":      "fee_lines",
		"coupon_lines":   "coupon_lines",
		"meta_data":      "meta_data",
	} {
		if err := woocommerce.SetJSONIfPresent(order, inputs, field, input); err != nil {
			return woocommerce.ErrorResult(err.Error()), nil
		}
	}
	if err := woocommerce.MergeAdditionalFields(order, inputs); err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}

	resp, err := woocommerce.CreateResource(auth, "/orders", order)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}
	out := woocommerce.ResourceResult(resp, "")
	out["tool_result"] = fmt.Sprintf("Created order %s", out["id"])
	return out, nil
}

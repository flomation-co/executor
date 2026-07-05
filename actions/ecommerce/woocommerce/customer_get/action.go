package ecommerce_woocommerce_customer_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	woocommerce "flomation.app/automate/executor/actions/ecommerce/woocommerce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WooCommerce: Get Customer"
	Description  = "Retrieve a single customer from your WooCommerce store by ID."
	Website      = "https://www.flomation.co"
	Icon         = "woocommerce+eye"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Store URL", Placeholder: "https://your-store.com — your store's root URL, not the /wp-json path", Required: true},
	{Name: "consumer_key", Type: core.ConnectionTypeSecret, Label: "Consumer Key", Placeholder: "ck_...", Required: true},
	{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Label: "Consumer Secret", Placeholder: "cs_...", Required: true},
	{Name: "credentials_in_query", Type: core.ConnectionTypeBoolean, Label: "Send Credentials in Query String", Placeholder: "Enable only if you see a \"Consumer key is missing\" error"},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID", Placeholder: "123", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Customer ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Customer"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := woocommerce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	customerID, err := woocommerce.RequiredString("customer_id", inputs)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}

	resp, err := woocommerce.GetResource(auth, "/customers/"+url.PathEscape(customerID), nil)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}
	out := woocommerce.ResourceResult(resp, fmt.Sprintf("Retrieved customer %s", customerID))
	return out, nil
}

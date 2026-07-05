package ecommerce_woocommerce_coupon_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	woocommerce "flomation.app/automate/executor/actions/ecommerce/woocommerce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WooCommerce: Get Coupon"
	Description  = "Retrieve a single coupon from your WooCommerce store by ID."
	Website      = "https://www.flomation.co"
	Icon         = "woocommerce+eye"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Store URL", Placeholder: "https://your-store.com", Required: true},
	{Name: "consumer_key", Type: core.ConnectionTypeSecret, Label: "Consumer Key", Placeholder: "ck_...", Required: true},
	{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Label: "Consumer Secret", Placeholder: "cs_...", Required: true},
	{Name: "credentials_in_query", Type: core.ConnectionTypeBoolean, Label: "Send Credentials in Query String", Placeholder: "Enable only if you see a \"Consumer key is missing\" error"},
	{Name: "coupon_id", Type: core.ConnectionTypeString, Label: "Coupon ID", Placeholder: "123", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Coupon ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Coupon"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := woocommerce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	couponID, err := woocommerce.RequiredString("coupon_id", inputs)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}

	resp, err := woocommerce.GetResource(auth, "/coupons/"+url.PathEscape(couponID), nil)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}
	out := woocommerce.ResourceResult(resp, fmt.Sprintf("Retrieved coupon %s", couponID))
	return out, nil
}

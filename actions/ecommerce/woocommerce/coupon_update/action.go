package ecommerce_woocommerce_coupon_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	woocommerce "flomation.app/automate/executor/actions/ecommerce/woocommerce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WooCommerce: Update Coupon"
	Description  = "Update an existing coupon in your WooCommerce store. Only the fields you set are changed; add any other coupon field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "woocommerce+pen"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Store URL", Placeholder: "https://your-store.com", Required: true},
	{Name: "consumer_key", Type: core.ConnectionTypeSecret, Label: "Consumer Key", Placeholder: "ck_...", Required: true},
	{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Label: "Consumer Secret", Placeholder: "cs_...", Required: true},
	{Name: "credentials_in_query", Type: core.ConnectionTypeBoolean, Label: "Send Credentials in Query String", Placeholder: "Enable only if you see a \"Consumer key is missing\" error"},
	{Name: "coupon_id", Type: core.ConnectionTypeString, Label: "Coupon ID", Placeholder: "123", Required: true},
	{Name: "code", Type: core.ConnectionTypeString, Label: "Coupon Code", Placeholder: "SAVE10"},
	{
		Name:  "discount_type",
		Type:  core.ConnectionTypeString,
		Label: "Discount Type",
		Options: []core.ConnectionOption{
			{Name: "Percentage", Value: "percent"},
			{Name: "Fixed Cart Discount", Value: "fixed_cart"},
			{Name: "Fixed Product Discount", Value: "fixed_product"},
		},
	},
	{Name: "amount", Type: core.ConnectionTypeString, Label: "Amount", Placeholder: "10 — a percentage or fixed amount depending on Discount Type"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description"},
	{Name: "date_expires", Type: core.ConnectionTypeDateTime, Label: "Expiry Date"},
	{Name: "minimum_amount", Type: core.ConnectionTypeString, Label: "Minimum Spend"},
	{Name: "maximum_amount", Type: core.ConnectionTypeString, Label: "Maximum Spend"},
	{Name: "individual_use", Type: core.ConnectionTypeBoolean, Label: "Individual Use Only", Placeholder: "Cannot be used in combination with any other coupon"},
	{Name: "exclude_sale_items", Type: core.ConnectionTypeBoolean, Label: "Exclude Sale Items"},
	{Name: "free_shipping", Type: core.ConnectionTypeBoolean, Label: "Allow Free Shipping"},
	{Name: "usage_limit", Type: core.ConnectionTypeInteger, Label: "Usage Limit", Placeholder: "How many times this coupon can be used in total"},
	{Name: "usage_limit_per_user", Type: core.ConnectionTypeInteger, Label: "Usage Limit Per User"},
	{Name: "limit_usage_to_x_items", Type: core.ConnectionTypeInteger, Label: "Limit Usage to X Items", Placeholder: "Max number of qualifying items the coupon can apply to"},
	{Name: "product_ids", Type: core.ConnectionTypeString, Label: "Product IDs", Placeholder: "Comma-separated product IDs the coupon applies to"},
	{Name: "excluded_product_ids", Type: core.ConnectionTypeString, Label: "Excluded Product IDs", Placeholder: "Comma-separated product IDs the coupon will not apply to"},
	{Name: "product_categories", Type: core.ConnectionTypeString, Label: "Product Category IDs", Placeholder: "Comma-separated category IDs the coupon applies to"},
	{Name: "excluded_product_categories", Type: core.ConnectionTypeString, Label: "Excluded Product Category IDs", Placeholder: "Comma-separated category IDs the coupon will not apply to"},
	{Name: "email_restrictions", Type: core.ConnectionTypeObject, Label: "Email Restrictions (JSON)", Placeholder: `["jane@example.com"]`},
	{Name: "meta_data", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `[{"key":"source","value":"web"}]`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)"},
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

	coupon := map[string]interface{}{}
	woocommerce.SetIfPresent(coupon, inputs, "code", "code")
	woocommerce.SetIfPresent(coupon, inputs, "discount_type", "discount_type")
	woocommerce.SetIfPresent(coupon, inputs, "amount", "amount")
	woocommerce.SetIfPresent(coupon, inputs, "description", "description")
	woocommerce.SetIfPresent(coupon, inputs, "date_expires", "date_expires")
	woocommerce.SetIfPresent(coupon, inputs, "minimum_amount", "minimum_amount")
	woocommerce.SetIfPresent(coupon, inputs, "maximum_amount", "maximum_amount")
	woocommerce.SetBoolIfSet(coupon, inputs, "individual_use", "individual_use")
	woocommerce.SetBoolIfSet(coupon, inputs, "exclude_sale_items", "exclude_sale_items")
	woocommerce.SetBoolIfSet(coupon, inputs, "free_shipping", "free_shipping")
	for field, input := range map[string]string{
		"usage_limit":            "usage_limit",
		"usage_limit_per_user":   "usage_limit_per_user",
		"limit_usage_to_x_items": "limit_usage_to_x_items",
	} {
		if err := woocommerce.SetIntIfPresent(coupon, inputs, field, input); err != nil {
			return woocommerce.ErrorResult(err.Error()), nil
		}
	}
	woocommerce.SetIntListIfPresent(coupon, inputs, "product_ids", "product_ids")
	woocommerce.SetIntListIfPresent(coupon, inputs, "excluded_product_ids", "excluded_product_ids")
	woocommerce.SetIntListIfPresent(coupon, inputs, "product_categories", "product_categories")
	woocommerce.SetIntListIfPresent(coupon, inputs, "excluded_product_categories", "excluded_product_categories")
	for field, input := range map[string]string{
		"email_restrictions": "email_restrictions",
		"meta_data":          "meta_data",
	} {
		if err := woocommerce.SetJSONIfPresent(coupon, inputs, field, input); err != nil {
			return woocommerce.ErrorResult(err.Error()), nil
		}
	}
	if err := woocommerce.MergeAdditionalFields(coupon, inputs); err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}

	resp, err := woocommerce.UpdateResource(auth, "/coupons/"+url.PathEscape(couponID), coupon)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}
	out := woocommerce.ResourceResult(resp, fmt.Sprintf("Updated coupon %s", couponID))
	return out, nil
}

package ecommerce_woocommerce_customer_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	woocommerce "flomation.app/automate/executor/actions/ecommerce/woocommerce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WooCommerce: Update Customer"
	Description  = "Update an existing customer in your WooCommerce store. Only the fields you set are changed; add any other customer field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "woocommerce+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Store URL", Placeholder: "https://your-store.com — your store's root URL, not the /wp-json path", Required: true},
	{Name: "consumer_key", Type: core.ConnectionTypeSecret, Label: "Consumer Key", Placeholder: "ck_...", Required: true},
	{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Label: "Consumer Secret", Placeholder: "cs_...", Required: true},
	{Name: "credentials_in_query", Type: core.ConnectionTypeBoolean, Label: "Send Credentials in Query String", Placeholder: "Enable only if you see a \"Consumer key is missing\" error"},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID", Placeholder: "123", Required: true},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "jane@example.com"},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password"},
	{
		Name:  "role",
		Type:  core.ConnectionTypeString,
		Label: "Role",
		Options: []core.ConnectionOption{
			{Name: "Customer", Value: "customer"},
			{Name: "Subscriber", Value: "subscriber"},
			{Name: "Contributor", Value: "contributor"},
			{Name: "Author", Value: "author"},
			{Name: "Editor", Value: "editor"},
			{Name: "Shop Manager", Value: "shop_manager"},
			{Name: "Administrator", Value: "administrator"},
		},
	},
	{Name: "billing", Type: core.ConnectionTypeObject, Label: "Billing Address (JSON)", Placeholder: `{"first_name":"Jane","address_1":"1 Main St","city":"London","postcode":"SW1","country":"GB","email":"jane@example.com"}`},
	{Name: "shipping", Type: core.ConnectionTypeObject, Label: "Shipping Address (JSON)", Placeholder: `{"first_name":"Jane","last_name":"Doe","address_1":"1 Main St","city":"London","postcode":"SW1","country":"GB"}`},
	{Name: "meta_data", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `[{"key":"vip","value":"yes"}]`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)"},
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

	customer := map[string]interface{}{}
	woocommerce.SetIfPresent(customer, inputs, "first_name", "first_name")
	woocommerce.SetIfPresent(customer, inputs, "last_name", "last_name")
	woocommerce.SetIfPresent(customer, inputs, "email", "email")
	woocommerce.SetIfPresent(customer, inputs, "password", "password")
	woocommerce.SetIfPresent(customer, inputs, "role", "role")
	for field, input := range map[string]string{
		"billing":   "billing",
		"shipping":  "shipping",
		"meta_data": "meta_data",
	} {
		if err := woocommerce.SetJSONIfPresent(customer, inputs, field, input); err != nil {
			return woocommerce.ErrorResult(err.Error()), nil
		}
	}
	if err := woocommerce.MergeAdditionalFields(customer, inputs); err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}

	resp, err := woocommerce.UpdateResource(auth, "/customers/"+url.PathEscape(customerID), customer)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}
	out := woocommerce.ResourceResult(resp, fmt.Sprintf("Updated customer %s", customerID))
	return out, nil
}

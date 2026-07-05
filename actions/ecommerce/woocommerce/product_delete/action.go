package ecommerce_woocommerce_product_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	woocommerce "flomation.app/automate/executor/actions/ecommerce/woocommerce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WooCommerce: Delete Product"
	Description  = "Delete a product from your WooCommerce store. Deletes permanently by default; turn off Force Delete to move it to the trash instead."
	Website      = "https://www.flomation.co"
	Icon         = "woocommerce+trash"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Store URL", Placeholder: "https://your-store.com", Required: true},
	{Name: "consumer_key", Type: core.ConnectionTypeSecret, Label: "Consumer Key", Placeholder: "ck_...", Required: true},
	{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Label: "Consumer Secret", Placeholder: "cs_...", Required: true},
	{Name: "credentials_in_query", Type: core.ConnectionTypeBoolean, Label: "Send Credentials in Query String", Placeholder: "Enable only if you see a \"Consumer key is missing\" error"},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product ID", Placeholder: "123", Required: true},
	{Name: "force", Type: core.ConnectionTypeBoolean, Label: "Force Delete", Placeholder: "On (default): delete permanently. Off: move to trash.", Value: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deleted Product"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := woocommerce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	productID, err := woocommerce.RequiredString("product_id", inputs)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}
	// Default to a permanent delete (matching n8n), but honour an explicit
	// Force Delete = false to trash instead. The input defaults to true, so an
	// untouched checkbox force-deletes.
	force := true
	if conn := core.FindConnection("force", inputs); conn != nil && conn.Boolean() != nil {
		force = *conn.Boolean()
	}

	resp, err := woocommerce.DeleteResource(auth, "/products/"+url.PathEscape(productID), force)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}
	verb := "Deleted"
	if !force {
		verb = "Trashed"
	}
	out := woocommerce.ResourceResult(resp, fmt.Sprintf("%s product %s", verb, productID))
	return out, nil
}

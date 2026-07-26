package crm_salesforce_product_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Product"
	Description  = "Send a product to the Salesforce Recycle Bin, where it can be restored for 15 days. Its prices go with it. Salesforce will refuse if the product is already on a deal - retire it with Update Product instead."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product", Placeholder: "01t5f000004AbCdAAK - the record ID of the product to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deleted Product"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	productID := salesforce.OptionalString("product_id", inputs)
	if productID == "" {
		return nil, fmt.Errorf("product_id is required — the record ID of the product to delete, e.g. 01t5f000004AbCdAAK")
	}
	if err := salesforce.ValidateRecordID(productID); err != nil {
		return nil, err
	}

	if err := salesforce.DeleteRecord(instanceURL, token, "Product2", productID); err != nil {
		// DELETE_FAILED is the outcome that actually happens in a real catalogue,
		// and common.go has no translation for it. Verified live: deleting a
		// product that sits on a deal answers 400 DELETE_FAILED "Your attempt to
		// delete <product> could not be completed because it is associated with
		// the following opportunity products.: <deal name>".
		//
		// Salesforce's own text names the deal, which is genuinely useful, but it
		// does not say what to do instead — and what to do is untick Ready To Sell,
		// which stops the product appearing on anything new while leaving the
		// history intact. That is the whole answer and it is one sentence.
		if salesforce.ErrorHasCode(err, "DELETE_FAILED") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"Salesforce will not delete this product because it is already used on a deal, quote or order — retire it instead by unticking Ready To Sell with Update Product, which stops it appearing on anything new without disturbing the records it is already on (%s)", err.Error())), nil
		}
		// Deleting a record that is already gone answers ENTITY_IS_DELETED, which
		// CheckResponse translates. It is a provider outcome, not a configuration
		// mistake, so it takes the error port as data.
		return salesforce.ErrorResult(err.Error()), nil
	}

	// A successful DELETE is 204 No Content, so the ID the operator supplied is
	// the only thing there is to return — and it is what a downstream node needs
	// in order to log or undo the deletion.
	//
	// Worth knowing, and verified live: deleting a product also deletes its price
	// book entries (both entries on a probe product came back IsDeleted=true), so
	// the prices are not left behind pointing at nothing.
	return salesforce.RecordResult(productID, map[string]interface{}{"Id": productID, "deleted": true}, fmt.Sprintf(
		"Sent product %s to the Recycle Bin, along with its price book entries", productID)), nil
}

package crm_salesforce_order_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Order"
	Description  = "Change an order in Salesforce - correct the delivery address, add the customer's PO number, move the dates, or put an activated order back to Draft so its products can be edited. Anything you leave blank is left exactly as it was."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "order_id", Type: core.ConnectionTypeString, Label: "Order ID", Placeholder: "8015f000000AbCdAAK - the order to change", Required: true},
	{
		Name:        "order_status",
		Type:        core.ConnectionTypeString,
		Label:       "Status",
		Placeholder: "Draft - putting an activated order back to Draft is how you unlock its products",
		Options: []core.ConnectionOption{
			{Name: "Draft", Value: "Draft"},
			{Name: "Activated", Value: "Activated"},
		},
	},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Company (Account)", Placeholder: "0015f00000AbCdEAAV - the customer this order is for"},
	{Name: "contract_id", Type: core.ConnectionTypeString, Label: "Contract", Placeholder: "8005f000000AbCdAAK - the contract this order is placed under"},
	{Name: "order_name", Type: core.ConnectionTypeString, Label: "Order Name", Placeholder: "Acme Ltd - August generator order"},
	{Name: "order_type", Type: core.ConnectionTypeString, Label: "Order Type", Placeholder: "Must match an Order Type in your Salesforce org - Salesforce ships none, so leave it blank unless an administrator has added some"},
	{Name: "effective_date", Type: core.ConnectionTypeDateTime, Label: "Order Start Date", Placeholder: "2026-08-01 (the date only - Salesforce ignores the time)"},
	{Name: "end_date", Type: core.ConnectionTypeDateTime, Label: "Order End Date", Placeholder: "2027-07-31 (the date only) - for a term or subscription order"},
	{Name: "po_number", Type: core.ConnectionTypeString, Label: "Customer PO Number", Placeholder: "PO-2026-4471 - the customer's own purchase order reference"},
	{Name: "po_date", Type: core.ConnectionTypeDateTime, Label: "Customer PO Date", Placeholder: "2026-07-28 (the date only)"},
	{Name: "order_reference_number", Type: core.ConnectionTypeString, Label: "Your Order Reference", Placeholder: "SHOP-10482 - your own reference, e.g. the Shopify or WooCommerce order number"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Anything the warehouse or finance need to know about this order"},
	{Name: "pricebook_id", Type: core.ConnectionTypeString, Label: "Price Book", Placeholder: "01s5f000004AbCdAAK - cannot be changed once the order has product lines"},
	{Name: "bill_to_contact_id", Type: core.ConnectionTypeString, Label: "Bill To Contact", Placeholder: "0035f00000AbCdEAAV - who to invoice"},
	{Name: "ship_to_contact_id", Type: core.ConnectionTypeString, Label: "Ship To Contact", Placeholder: "0035f00000AbCdEAAV - who receives the goods"},
	{Name: "billing_street", Type: core.ConnectionTypeText, Label: "Bill To Street", Placeholder: "1 High Street"},
	{Name: "billing_city", Type: core.ConnectionTypeString, Label: "Bill To City", Placeholder: "London"},
	{Name: "billing_state", Type: core.ConnectionTypeString, Label: "Bill To State/Province", Placeholder: "Only for countries your Salesforce org lists states for - the United Kingdom has none, so leave it blank"},
	{Name: "billing_postal_code", Type: core.ConnectionTypeString, Label: "Bill To Postcode", Placeholder: "EC1A 1BB"},
	{Name: "billing_country", Type: core.ConnectionTypeString, Label: "Bill To Country", Placeholder: "United Kingdom - must match your org's country list"},
	{Name: "shipping_street", Type: core.ConnectionTypeText, Label: "Ship To Street", Placeholder: "Unit 4, Riverside Estate"},
	{Name: "shipping_city", Type: core.ConnectionTypeString, Label: "Ship To City", Placeholder: "Manchester"},
	{Name: "shipping_state", Type: core.ConnectionTypeString, Label: "Ship To State/Province", Placeholder: "Only for countries your Salesforce org lists states for - the United Kingdom has none, so leave it blank"},
	{Name: "shipping_postal_code", Type: core.ConnectionTypeString, Label: "Ship To Postcode", Placeholder: "M1 2AB"},
	{Name: "shipping_country", Type: core.ConnectionTypeString, Label: "Ship To Country", Placeholder: "United Kingdom - must match your org's country list"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "0055f00000AbCdEAAV - the person who owns this order"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other Salesforce field on the order"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Order ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Applied Changes"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	id := salesforce.OptionalString("order_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// Every field is optional and every one goes through Set*IfPresent: an update
	// that posted all its blank inputs would clear the operator's data, which on a
	// live order means wiping the delivery address and the PO number because they
	// only wanted to move a date.
	body := map[string]interface{}{}

	// Order.Status is a RESTRICTED picklist offering exactly Draft and Activated
	// (read from the live org's describe), so Salesforce refuses anything else
	// itself — but with "bad value for restricted picklist field", which does not
	// tell an operator what the good values are. Say it here instead.
	if status := salesforce.OptionalString("order_status", inputs); status != "" {
		switch strings.ToLower(status) {
		case "draft":
			body["Status"] = "Draft"
		case "activated":
			body["Status"] = "Activated"
		default:
			return nil, fmt.Errorf("%q is not a Salesforce order status — Order Status offers only Draft and Activated. Cancelled, Expired and Superseded are set by Salesforce's own order processes, not by an update", status)
		}
	}

	salesforce.SetIfPresent(body, inputs, "AccountId", "account_id")
	salesforce.SetIfPresent(body, inputs, "ContractId", "contract_id")
	salesforce.SetIfPresent(body, inputs, "Name", "order_name")
	salesforce.SetIfPresent(body, inputs, "Type", "order_type")
	// EffectiveDate, EndDate and PoDate are Date fields, not DateTimes — a full
	// ISO timestamp is rejected outright.
	salesforce.SetDateIfPresent(body, inputs, "EffectiveDate", "effective_date")
	salesforce.SetDateIfPresent(body, inputs, "EndDate", "end_date")
	salesforce.SetDateIfPresent(body, inputs, "PoDate", "po_date")
	salesforce.SetIfPresent(body, inputs, "PoNumber", "po_number")
	salesforce.SetIfPresent(body, inputs, "OrderReferenceNumber", "order_reference_number")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "Pricebook2Id", "pricebook_id")
	salesforce.SetIfPresent(body, inputs, "BillToContactId", "bill_to_contact_id")
	salesforce.SetIfPresent(body, inputs, "ShipToContactId", "ship_to_contact_id")
	salesforce.SetIfPresent(body, inputs, "BillingStreet", "billing_street")
	salesforce.SetIfPresent(body, inputs, "BillingCity", "billing_city")
	salesforce.SetIfPresent(body, inputs, "BillingState", "billing_state")
	salesforce.SetIfPresent(body, inputs, "BillingPostalCode", "billing_postal_code")
	salesforce.SetIfPresent(body, inputs, "BillingCountry", "billing_country")
	salesforce.SetIfPresent(body, inputs, "ShippingStreet", "shipping_street")
	salesforce.SetIfPresent(body, inputs, "ShippingCity", "shipping_city")
	salesforce.SetIfPresent(body, inputs, "ShippingState", "shipping_state")
	salesforce.SetIfPresent(body, inputs, "ShippingPostalCode", "shipping_postal_code")
	salesforce.SetIfPresent(body, inputs, "ShippingCountry", "shipping_country")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change on the order")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Order", id, body); err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"orders are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Order Settings ▸ Enable Orders (%s)", err.Error())), nil
		}
		if salesforce.ErrorHasCode(err, "ENTITY_IS_LOCKED") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"that order is activated, so Salesforce has locked this change — set its Status back to Draft first (this same action), make the change, then activate it again (%s)", err.Error())), nil
		}
		// FAILED_ACTIVATION is what an operator gets for setting Status to Activated
		// on an order with no product lines. Salesforce's own sentence is fine as far
		// as it goes ("An order must have at least one product.") but it names no
		// remedy, and four sibling actions — Activate Order among them — already
		// translate this exact code. Say which step adds the products.
		if salesforce.ErrorHasCode(err, "FAILED_ACTIVATION") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"Salesforce would not activate that order — an order needs at least one product line before it can be activated, so add its products first with Add Product to Order (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers an update with 204 No Content — there is no updated
	// record to return. Echo back what was actually applied (plus the ID) so the
	// next node has something to work with and the execution view shows what
	// changed. Use the Get Order action if the full record is needed.
	changed := salesforce.SortedKeys(body)
	record := make(map[string]interface{}, len(body)+1)
	for k, v := range body {
		record[k] = v
	}
	record["Id"] = id

	return salesforce.RecordResult(id, record, fmt.Sprintf("Updated order %s — changed %s", id, strings.Join(changed, ", "))), nil
}

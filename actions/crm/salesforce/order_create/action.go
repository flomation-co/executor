package crm_salesforce_order_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Order"
	Description  = "Raise an order against a customer's account in Salesforce - the record that turns a won deal or an online sale into something the warehouse and finance can work from. Add its product lines next, then activate it."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clipboard-list"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Company (Account)", Placeholder: "0015f00000AbCdEAAV - the customer this order is for", Required: true},
	{Name: "effective_date", Type: core.ConnectionTypeDateTime, Label: "Order Start Date", Placeholder: "2026-08-01 (the date only - Salesforce ignores the time)", Required: true},
	{
		Name:        "order_status",
		Type:        core.ConnectionTypeString,
		Label:       "Status",
		Placeholder: "Draft - a new order always starts as a draft (leave blank for Draft)",
		Options: []core.ConnectionOption{
			{Name: "Draft", Value: "Draft"},
			{Name: "Activated", Value: "Activated"},
		},
	},
	{Name: "pricebook_id", Type: core.ConnectionTypeString, Label: "Price Book", Placeholder: "01s5f000004AbCdAAK - the price list to order from; needed before any product can be added"},
	{Name: "contract_id", Type: core.ConnectionTypeString, Label: "Contract", Placeholder: "8005f000000AbCdAAK - the contract this order is placed under"},
	{Name: "order_name", Type: core.ConnectionTypeString, Label: "Order Name", Placeholder: "Acme Ltd - August generator order (Salesforce numbers the order for you)"},
	{Name: "order_type", Type: core.ConnectionTypeString, Label: "Order Type", Placeholder: "Must match an Order Type in your Salesforce org - Salesforce ships none, so leave it blank unless an administrator has added some"},
	{Name: "end_date", Type: core.ConnectionTypeDateTime, Label: "Order End Date", Placeholder: "2027-07-31 (the date only) - for a term or subscription order"},
	{Name: "po_number", Type: core.ConnectionTypeString, Label: "Customer PO Number", Placeholder: "PO-2026-4471 - the customer's own purchase order reference"},
	{Name: "po_date", Type: core.ConnectionTypeDateTime, Label: "Customer PO Date", Placeholder: "2026-07-28 (the date only)"},
	{Name: "order_reference_number", Type: core.ConnectionTypeString, Label: "Your Order Reference", Placeholder: "SHOP-10482 - your own reference, e.g. the Shopify or WooCommerce order number"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Anything the warehouse or finance need to know about this order"},
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
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// Account is required in practice even though the describe reports the field
	// as nillable — verified live, an order without one is REQUIRED_FIELD_MISSING
	// with the message "Select an account." and no field name attached, which is
	// impossible to act on. So is the start date. Checking both here names the box
	// the operator left empty.
	accountID := salesforce.OptionalString("account_id", inputs)
	if accountID == "" {
		return nil, fmt.Errorf("the company is required — every Salesforce order belongs to an account, e.g. 0015f00000AbCdEAAV")
	}
	if err := salesforce.ValidateRecordID(accountID); err != nil {
		return nil, err
	}
	if salesforce.OptionalString("effective_date", inputs) == "" {
		return nil, fmt.Errorf("the order start date is required — Salesforce will not create an order without one, e.g. 2026-08-01")
	}

	// A new order can only be a Draft. Verified live: creating one with
	// Status "Activated" fails FAILED_ACTIVATION ("For a new or cloned order,
	// choose Draft"), because activation is a transition Salesforce performs on an
	// order that already has product lines — and it cannot have any yet. Saying so
	// here points at the Activate Order action instead of leaving the operator
	// staring at a status they were offered and cannot use.
	status := salesforce.OptionalString("order_status", inputs)
	switch strings.ToLower(status) {
	case "", "draft":
		status = "Draft"
	case "activated":
		return nil, fmt.Errorf("a new Salesforce order has to start as a Draft — create it, add its product lines, then use the Activate Order action. An order with no products cannot be activated at all")
	default:
		return nil, fmt.Errorf("%q is not a Salesforce order status — Order Status is a restricted picklist offering only Draft and Activated, and a new order must be Draft", status)
	}

	body := map[string]interface{}{
		"AccountId": accountID,
		"Status":    status,
	}
	// EffectiveDate and EndDate are Date fields, not DateTimes — a full ISO
	// timestamp from an upstream date picker is rejected outright.
	salesforce.SetDateIfPresent(body, inputs, "EffectiveDate", "effective_date")
	salesforce.SetDateIfPresent(body, inputs, "EndDate", "end_date")
	salesforce.SetDateIfPresent(body, inputs, "PoDate", "po_date")

	// Every optional field goes through Set*IfPresent so an untouched input is
	// omitted rather than sent blank — Salesforce reads an explicit empty value as
	// "clear this field".
	salesforce.SetIfPresent(body, inputs, "Pricebook2Id", "pricebook_id")
	salesforce.SetIfPresent(body, inputs, "ContractId", "contract_id")
	salesforce.SetIfPresent(body, inputs, "Name", "order_name")
	salesforce.SetIfPresent(body, inputs, "Type", "order_type")
	salesforce.SetIfPresent(body, inputs, "PoNumber", "po_number")
	salesforce.SetIfPresent(body, inputs, "OrderReferenceNumber", "order_reference_number")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
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

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Order", body)
	if err != nil {
		// Orders is a per-org feature that is OFF in a stock org, so INVALID_TYPE
		// here has one overwhelmingly likely cause and one specific fix.
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"orders are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Order Settings ▸ Enable Orders (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Created draft order for account %s (%s)", accountID, id)
	if salesforce.OptionalString("pricebook_id", inputs) == "" {
		// Salesforce does NOT fill the order's price book in for you — verified
		// live, a new order's Pricebook2Id comes back null — and without one no
		// product line can be added. Say it now rather than let the next step in
		// the flow discover it.
		summary += " — no price book was set, so add one before adding products"
	}
	return salesforce.RecordResult(id, raw, summary), nil
}

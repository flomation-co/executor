package crm_salesforce_quote_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Quote"
	Description  = "Change a quote in Salesforce - mark it Presented or Accepted, push the expiry out, correct the tax or the delivery address. Anything you leave blank is left exactly as it was."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

// quoteStatuses is the standard Quote Status list, read from the live org's
// describe.
//
// Quote.Status is an UNRESTRICTED picklist, which is why this map exists.
// Verified live: setting Status to "Totally Made Up" answers success and stores
// that string verbatim. On an update that is worse than on a create — the quote
// already existed, already appeared in the right reports, and a single misspelled
// status quietly drops it out of every one of them.
var quoteStatuses = map[string]string{
	"draft":        "Draft",
	"needs review": "Needs Review",
	"in review":    "In Review",
	"approved":     "Approved",
	"rejected":     "Rejected",
	"presented":    "Presented",
	"accepted":     "Accepted",
	"denied":       "Denied",
}

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "quote_id", Type: core.ConnectionTypeString, Label: "Quote ID", Placeholder: "0Q05f000000AbCdAAK - the quote to change", Required: true},
	{
		Name:        "quote_status",
		Type:        core.ConnectionTypeString,
		Label:       "Status",
		Placeholder: "Presented - where the quote now sits in your approval process",
		Options: []core.ConnectionOption{
			{Name: "Draft", Value: "Draft"},
			{Name: "Needs Review", Value: "Needs Review"},
			{Name: "In Review", Value: "In Review"},
			{Name: "Approved", Value: "Approved"},
			{Name: "Rejected", Value: "Rejected"},
			{Name: "Presented", Value: "Presented"},
			{Name: "Accepted", Value: "Accepted"},
			{Name: "Denied", Value: "Denied"},
		},
	},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Quote Name", Placeholder: "Acme Ltd - 50 seat renewal (June revision)"},
	{Name: "expiration_date", Type: core.ConnectionTypeDateTime, Label: "Expires On", Placeholder: "2026-10-15 (the date only - Salesforce ignores the time)"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Quote Recipient (Contact)", Placeholder: "0035f00000AbCdEAAV - the person the quote is addressed to"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Recipient Email", Placeholder: "buyer@acme.example.com - where the quote gets sent"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Recipient Phone", Placeholder: "020 7946 0000"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Notes about this quote, visible to everyone on the deal"},
	{Name: "tax", Type: core.ConnectionTypeString, Label: "Tax", Placeholder: "250.50 - the tax amount added to the quote total"},
	{Name: "shipping_handling", Type: core.ConnectionTypeString, Label: "Shipping & Handling", Placeholder: "45.00 - carriage added to the quote total"},
	{Name: "pricebook_id", Type: core.ConnectionTypeString, Label: "Price Book", Placeholder: "01s5f000004AbCdAAK - cannot be changed once the quote has product lines"},
	{Name: "opportunity_id", Type: core.ConnectionTypeString, Label: "Opportunity (Deal)", Placeholder: "0065f00000AbCdEAAV - the deal this quote belongs to"},
	{Name: "contract_id", Type: core.ConnectionTypeString, Label: "Contract", Placeholder: "8005f000000AbCdAAK - the contract this quote relates to"},
	{Name: "billing_name", Type: core.ConnectionTypeString, Label: "Bill To Name", Placeholder: "Acme Ltd - Accounts Payable"},
	{Name: "billing_street", Type: core.ConnectionTypeText, Label: "Bill To Street", Placeholder: "1 High Street"},
	{Name: "billing_city", Type: core.ConnectionTypeString, Label: "Bill To City", Placeholder: "London"},
	{Name: "billing_state", Type: core.ConnectionTypeString, Label: "Bill To State/Province", Placeholder: "Only for countries your Salesforce org lists states for - the United Kingdom has none, so leave it blank"},
	{Name: "billing_postal_code", Type: core.ConnectionTypeString, Label: "Bill To Postcode", Placeholder: "EC1A 1BB"},
	{Name: "billing_country", Type: core.ConnectionTypeString, Label: "Bill To Country", Placeholder: "United Kingdom - must match your org's country list"},
	{Name: "shipping_name", Type: core.ConnectionTypeString, Label: "Ship To Name", Placeholder: "Acme Ltd - Goods In"},
	{Name: "shipping_street", Type: core.ConnectionTypeText, Label: "Ship To Street", Placeholder: "Unit 4, Riverside Estate"},
	{Name: "shipping_city", Type: core.ConnectionTypeString, Label: "Ship To City", Placeholder: "Manchester"},
	{Name: "shipping_state", Type: core.ConnectionTypeString, Label: "Ship To State/Province", Placeholder: "Only for countries your Salesforce org lists states for - the United Kingdom has none, so leave it blank"},
	{Name: "shipping_postal_code", Type: core.ConnectionTypeString, Label: "Ship To Postcode", Placeholder: "M1 2AB"},
	{Name: "shipping_country", Type: core.ConnectionTypeString, Label: "Ship To Country", Placeholder: "United Kingdom - must match your org's country list"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "0055f00000AbCdEAAV - the salesperson who owns the quote"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other Salesforce field on the quote"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Quote ID"},
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

	id := salesforce.OptionalString("quote_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// Every field is optional and every one goes through Set*IfPresent: an update
	// that posted all its blank inputs would clear the operator's data, which on a
	// live quote means wiping the delivery address and the tax because they only
	// wanted to mark it Presented.
	body := map[string]interface{}{}

	status, err := resolveStatus(salesforce.OptionalString("quote_status", inputs))
	if err != nil {
		return nil, err
	}
	if status != "" {
		body["Status"] = status
	}

	salesforce.SetIfPresent(body, inputs, "Name", "name")
	// ExpirationDate is a Date field — a full ISO timestamp is rejected outright.
	salesforce.SetDateIfPresent(body, inputs, "ExpirationDate", "expiration_date")
	salesforce.SetIfPresent(body, inputs, "ContactId", "contact_id")
	salesforce.SetIfPresent(body, inputs, "Email", "email")
	salesforce.SetIfPresent(body, inputs, "Phone", "phone")
	salesforce.SetIfPresent(body, inputs, "Description", "description")

	tax, taxSet, err := salesforce.NumericInput("tax", "Tax", "250.50", inputs)
	if err != nil {
		return nil, err
	}
	if taxSet {
		body["Tax"] = tax
	}
	carriage, carriageSet, err := salesforce.NumericInput("shipping_handling", "Shipping & Handling", "250.50", inputs)
	if err != nil {
		return nil, err
	}
	if carriageSet {
		body["ShippingHandling"] = carriage
	}

	salesforce.SetIfPresent(body, inputs, "Pricebook2Id", "pricebook_id")
	salesforce.SetIfPresent(body, inputs, "OpportunityId", "opportunity_id")
	salesforce.SetIfPresent(body, inputs, "ContractId", "contract_id")
	salesforce.SetIfPresent(body, inputs, "BillingName", "billing_name")
	salesforce.SetIfPresent(body, inputs, "BillingStreet", "billing_street")
	salesforce.SetIfPresent(body, inputs, "BillingCity", "billing_city")
	salesforce.SetIfPresent(body, inputs, "BillingState", "billing_state")
	salesforce.SetIfPresent(body, inputs, "BillingPostalCode", "billing_postal_code")
	salesforce.SetIfPresent(body, inputs, "BillingCountry", "billing_country")
	salesforce.SetIfPresent(body, inputs, "ShippingName", "shipping_name")
	salesforce.SetIfPresent(body, inputs, "ShippingStreet", "shipping_street")
	salesforce.SetIfPresent(body, inputs, "ShippingCity", "shipping_city")
	salesforce.SetIfPresent(body, inputs, "ShippingState", "shipping_state")
	salesforce.SetIfPresent(body, inputs, "ShippingPostalCode", "shipping_postal_code")
	salesforce.SetIfPresent(body, inputs, "ShippingCountry", "shipping_country")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")

	// Discount is deliberately NOT an input here. Quote.Discount is read-only —
	// verified live, PATCH {"Discount":10} answers INVALID_FIELD_FOR_INSERT_UPDATE
	// — because Salesforce computes it from the discounts on the quote's product
	// lines. To discount a quote, discount its lines. Additional Fields cannot
	// route round it either, which is why it is called out rather than left out.
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change on the quote")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Quote", id, body); err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"quotes are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Quotes ▸ Quote Settings (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers an update with 204 No Content — there is no updated
	// record to return. Echo back what was actually applied (plus the ID) so the
	// next node has something to work with and the execution view shows what
	// changed. Use the Get Quote action if the full record is needed.
	changed := salesforce.SortedKeys(body)
	record := make(map[string]interface{}, len(body)+1)
	for k, v := range body {
		record[k] = v
	}
	record["Id"] = id

	return salesforce.RecordResult(id, record, fmt.Sprintf("Updated quote %s — changed %s", id, strings.Join(changed, ", "))), nil
}

// resolveStatus maps the operator's status onto the exact Salesforce spelling,
// refusing anything outside the standard list. See quoteStatuses for why a
// refusal is kinder than the silent success Salesforce would otherwise give.
func resolveStatus(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if v, ok := quoteStatuses[strings.ToLower(raw)]; ok {
		return v, nil
	}
	return "", fmt.Errorf("%q is not a Salesforce quote status — choose Draft, Needs Review, In Review, Approved, Rejected, Presented, Accepted or Denied. If your org has added its own statuses, set Status through Additional Fields instead", raw)
}

// numericInput reads a decimal input, treating an unparseable value as the
// configuration mistake it is rather than silently dropping the field.
//

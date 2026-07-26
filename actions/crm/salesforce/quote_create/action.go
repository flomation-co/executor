package crm_salesforce_quote_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Quote"
	Description  = "Create a quote on a deal in Salesforce, ready for product lines to be added and sent to the customer. Salesforce works out the quote number, the customer's account and the totals for you."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+plus"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

// quoteStatuses is the standard Quote Status list, read from the live org's
// describe.
//
// Quote.Status is an UNRESTRICTED picklist, and that is the whole reason this map
// exists. Verified live: POST Quote {"Status":"Totally Made Up"} answers 201
// success and stores that string verbatim. No error, no warning — and from then
// on the quote matches no report, no approval process and no "quotes awaiting
// acceptance" filter anyone builds, because its status is a value Salesforce has
// never heard of. A restricted picklist (Order.Status, Opportunity.StageName)
// rejects junk itself; this one cannot, so it is checked here or nowhere.
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
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Quote Name", Placeholder: "Acme Ltd - 50 seat renewal (June revision)", Required: true},
	{Name: "opportunity_id", Type: core.ConnectionTypeString, Label: "Opportunity (Deal)", Placeholder: "0065f00000AbCdEAAV - the deal this quote is for; Salesforce copies the customer's account across from it. Needed unless an administrator has switched on Create Quotes Without a Related Opportunity"},
	{Name: "pricebook_id", Type: core.ConnectionTypeString, Label: "Price Book", Placeholder: "01s5f000004AbCdAAK - the price list to quote from; needed before any product can be added"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Quote Recipient (Contact)", Placeholder: "0035f00000AbCdEAAV - the person the quote is addressed to"},
	{
		Name:        "quote_status",
		Type:        core.ConnectionTypeString,
		Label:       "Status",
		Placeholder: "Draft - where the quote sits in your approval process (leave blank for Draft)",
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
	{Name: "expiration_date", Type: core.ConnectionTypeDateTime, Label: "Expires On", Placeholder: "2026-09-30 (the date only - Salesforce ignores the time)"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Notes about this quote, visible to everyone on the deal"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Recipient Email", Placeholder: "buyer@acme.example.com - where the quote gets sent"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Recipient Phone", Placeholder: "020 7946 0000"},
	{Name: "tax", Type: core.ConnectionTypeString, Label: "Tax", Placeholder: "250.50 - the tax amount to add to the quote total"},
	{Name: "shipping_handling", Type: core.ConnectionTypeString, Label: "Shipping & Handling", Placeholder: "45.00 - carriage to add to the quote total"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Company (Quote Account)", Placeholder: "0015f00000AbCdEAAV - only needed for a quote with no deal behind it"},
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

	// Name is the only field that is unconditionally required — and it is the only
	// one that can be checked here, because whether a quote may exist without a
	// deal is an ORG SETTING rather than an API rule. In the live test org
	// "Create Quotes Without a Related Opportunity" is on, Quote.OpportunityId
	// describes as nillable, and a quote carrying nothing but a Name is accepted
	// (verified live: 201). Marking the Opportunity input Required would therefore
	// refuse a call the operator's own org allows, so the shortfall is explained
	// after the fact instead — see the REQUIRED_FIELD_MISSING branch below.
	name := salesforce.OptionalString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("the quote name is required — what this quote is called in Salesforce, e.g. \"Acme Ltd - 50 seat renewal\"")
	}

	body := map[string]interface{}{"Name": name}

	status, err := resolveStatus(salesforce.OptionalString("quote_status", inputs))
	if err != nil {
		return nil, err
	}
	if status != "" {
		body["Status"] = status
	}

	// Every optional field goes through Set*IfPresent so an untouched input is
	// omitted rather than sent blank — Salesforce reads an explicit empty value
	// as "clear this field".
	salesforce.SetIfPresent(body, inputs, "OpportunityId", "opportunity_id")
	salesforce.SetIfPresent(body, inputs, "Pricebook2Id", "pricebook_id")
	salesforce.SetIfPresent(body, inputs, "ContactId", "contact_id")
	// ExpirationDate is a Date field, not a DateTime — a full ISO timestamp from
	// an upstream date picker is rejected outright, so trim it to YYYY-MM-DD.
	salesforce.SetDateIfPresent(body, inputs, "ExpirationDate", "expiration_date")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "Email", "email")
	salesforce.SetIfPresent(body, inputs, "Phone", "phone")

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

	// QuoteAccountId, not AccountId: Quote.AccountId is read-only and Salesforce
	// derives it from the opportunity (verified live — a quote created with
	// QuoteAccountId set still reports AccountId as null). Writing to the
	// read-only one is an immediate INVALID_FIELD_FOR_INSERT_UPDATE.
	salesforce.SetIfPresent(body, inputs, "QuoteAccountId", "account_id")
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

	// Custom fields are the normal path on Quote, not an edge case — quote
	// templates almost always carry org-specific terms and reference numbers.
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Quote", body)
	if err != nil {
		// Quotes is a per-org feature that is OFF in a stock org, so INVALID_TYPE
		// here has one overwhelmingly likely cause and one specific fix.
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"quotes are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Quotes ▸ Quote Settings, then add the Quotes related list to the Opportunity page layout (%s)", err.Error())), nil
		}
		// A stock org has "Create Quotes Without a Related Opportunity" OFF, and
		// there this same call fails with REQUIRED_FIELD_MISSING naming
		// OpportunityId — an API field name, and no hint that the setting which
		// would let them leave it blank exists at all. The Opportunity input cannot
		// simply be marked Required to head this off, because in an org with the
		// setting on it genuinely is not. So name both ways out.
		if salesforce.ErrorHasCode(err, "REQUIRED_FIELD_MISSING") && strings.Contains(err.Error(), "OpportunityId") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"your Salesforce org will not create a quote that is not attached to a deal — either fill in the Opportunity, or ask an administrator to switch on Setup ▸ Quotes ▸ Quote Settings ▸ Create Quotes Without a Related Opportunity (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	opp := salesforce.OptionalString("opportunity_id", inputs)
	summary := fmt.Sprintf("Created quote %q (%s)", name, id)
	if opp != "" {
		summary = fmt.Sprintf("Created quote %q on opportunity %s (%s)", name, opp, id)
	}
	if salesforce.OptionalString("pricebook_id", inputs) == "" {
		// A quote with no price book cannot take a single product line — the
		// insert fails with FIELD_INTEGRITY_EXCEPTION (verified live). Say so
		// now rather than letting the next step in the flow discover it.
		//
		// But only the INPUT is known to be blank, not the record: when the quote is
		// raised on a deal, Salesforce copies the DEAL's price book onto it at insert
		// (verified live — the quote came back on the opportunity's own book and took
		// a product line straight away). Told flatly that no price book was set, an
		// operator goes hunting for a step they do not need, or sets a different book
		// with Update Quote — which is how a quote ends up priced off the wrong list,
		// since Salesforce does not require the two to match. So the note is only
		// stated as a fact when there is no deal to inherit from.
		if opp == "" {
			summary += " — no price book was set, so add one before adding products"
		} else {
			summary += " — no price book was set here, so the quote takes the deal's own price book; if that deal has none, set one before adding products"
		}
	}
	return salesforce.RecordResult(id, raw, summary), nil
}

// resolveStatus maps the operator's status onto the exact Salesforce spelling,
// refusing anything outside the standard list. See quoteStatuses for why a
// refusal is kinder than the silent 201 Salesforce would otherwise give.
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

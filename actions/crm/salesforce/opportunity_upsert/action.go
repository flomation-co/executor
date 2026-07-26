package crm_salesforce_opportunity_upsert

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create or Update Opportunity"
	Description  = "Match a deal on one of your own reference numbers and update it if it exists, or create it if it does not. This is what stops a re-run from creating a second copy of every deal."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+arrow-right-arrow-left"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "external_id_field", Type: core.ConnectionTypeString, Label: "Match On Field", Placeholder: "Order_Reference__c - a Salesforce field marked as an External ID", Required: true},
	{Name: "external_id_value", Type: core.ConnectionTypeString, Label: "Match On Value", Placeholder: "SO-10432 - the value to look for in that field", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Opportunity Name", Placeholder: "Acme Ltd - 50 seat renewal (required when the deal does not exist yet)"},
	{Name: "close_date", Type: core.ConnectionTypeDateTime, Label: "Expected Close Date", Placeholder: "2026-09-30 (required when the deal does not exist yet)"},
	{Name: "stage_name", Type: core.ConnectionTypeString, Label: "Stage", Placeholder: "Prospecting (required when the deal does not exist yet)"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Company (Account)", Placeholder: "0015f00000AbCdEAAV - the Salesforce account this deal belongs to"},
	{Name: "amount", Type: core.ConnectionTypeString, Label: "Amount", Placeholder: "12500.00 - the deal value in your org's currency"},
	{Name: "probability", Type: core.ConnectionTypeInteger, Label: "Probability (%)", Placeholder: "40 - leave blank to let Salesforce derive it from the stage"},
	{Name: "opportunity_type", Type: core.ConnectionTypeString, Label: "Opportunity Type", Placeholder: "New Customer, Existing Customer - Upgrade - must match your org's Opportunity Type list"},
	{Name: "lead_source", Type: core.ConnectionTypeString, Label: "Lead Source", Placeholder: "Web, Partner, Trade Show - must match your org's Lead Source list"},
	{Name: "next_step", Type: core.ConnectionTypeString, Label: "Next Step", Placeholder: "Send the proposal by Friday"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Background on the deal, visible to everyone on the account"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign", Placeholder: "7015f000000AbCdAAK - the campaign that sourced this deal"},
	{Name: "pricebook_id", Type: core.ConnectionTypeString, Label: "Price Book", Placeholder: "01s5f000004AbCdAAK - needed before you can add products to the deal"},
	{Name: "forecast_category", Type: core.ConnectionTypeString, Label: "Forecast Category", Placeholder: "Pipeline, Best Case, Commit, Closed - normally derived from the stage"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "0055f00000AbCdEAAV - the salesperson who owns the deal"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "0125f000000AbCdAAK - only if your org uses record types"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other Salesforce field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Opportunity ID"},
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

	extField := salesforce.OptionalString("external_id_field", inputs)
	if extField == "" {
		return nil, fmt.Errorf("the match-on field is required — pick a Salesforce field marked as an External ID, e.g. Order_Reference__c")
	}
	// The match field is an identifier: it goes into the URL path, where nothing
	// can quote it. Rejecting it here is also what makes it a HARD error, the way
	// record_upsert and account_upsert already treat it — a mistyped field name
	// ("Order Reference__c", with the space) is a configuration mistake that
	// never reaches Salesforce, and routing it to the soft error port instead
	// reads as though Salesforce had refused a well-formed request, so any retry
	// wired to that branch runs forever on something no retry can fix.
	if _, err := salesforce.ValidateSOQLFieldName(extField); err != nil {
		return nil, fmt.Errorf("Match On Field — %w", err)
	}
	extValue := salesforce.OptionalString("external_id_value", inputs)
	if extValue == "" {
		return nil, fmt.Errorf("the match-on value is required — it is what Salesforce looks for in %s", extField)
	}

	// Unlike create, none of Name/CloseDate/StageName is forced here. n8n reuses
	// its create form for upsert and therefore rewrites all three on every run,
	// silently resetting a deal's stage back to whatever the flow was built
	// with. Leaving them blank sends nothing and leaves the existing values
	// alone; Salesforce asks for them itself if the record has to be created.
	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Name", "name")
	salesforce.SetIfPresent(body, inputs, "StageName", "stage_name")
	// CloseDate is a Date field — a full ISO timestamp is rejected outright.
	salesforce.SetDateIfPresent(body, inputs, "CloseDate", "close_date")
	salesforce.SetIfPresent(body, inputs, "AccountId", "account_id")
	amount, amountSet, err := salesforce.NumericInput("amount", "Amount", "12500.00", inputs)
	if err != nil {
		return nil, err
	}
	if amountSet {
		body["Amount"] = amount
	}
	salesforce.SetIntIfPresent(body, inputs, "Probability", "probability")
	salesforce.SetIfPresent(body, inputs, "Type", "opportunity_type")
	salesforce.SetIfPresent(body, inputs, "LeadSource", "lead_source")
	salesforce.SetIfPresent(body, inputs, "NextStep", "next_step")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "CampaignId", "campaign_id")
	salesforce.SetIfPresent(body, inputs, "Pricebook2Id", "pricebook_id")
	salesforce.SetIfPresent(body, inputs, "ForecastCategoryName", "forecast_category")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	// Record Type is opt-in like every other field here — set it and the deal
	// gets it, leave it blank and a matched deal keeps whatever type it has. It
	// was simply missing: an operator who built Create Opportunity with a record
	// type and then swapped in this action to make the step re-runnable got the
	// profile's DEFAULT record type on every deal, with nothing said about it.
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	// UpsertRecord strips the match field from the body for us — Salesforce
	// rejects a payload that also sets the field named in the URL.
	id, created, raw, err := salesforce.UpsertRecord(instanceURL, token, "Opportunity", extField, extValue, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// An upsert that matched an existing record can answer 204 No Content, which
	// leaves us with no record ID to hand downstream. The whole point of upsert
	// is that the next node can carry on working with the deal, so resolve the
	// ID with one cheap lookup on the same external ID rather than emitting a
	// blank. A failure here is not fatal — the write already succeeded.
	if id == "" {
		if resolved := lookupID(instanceURL, token, extField, extValue); resolved != "" {
			id = resolved
		}
	}

	record := raw
	if record == nil {
		record = map[string]interface{}{}
	}
	record["created"] = created

	verb := "Updated"
	if created {
		verb = "Created"
	}
	return salesforce.RecordResult(id, record, fmt.Sprintf("%s opportunity matched on %s = %q (%s)", verb, extField, extValue, id)), nil
}

// numericInput reads a decimal input, treating an unparseable value as the
// configuration mistake it is rather than silently dropping the field.
//

// lookupID resolves the record ID of an opportunity by its external ID, for the
// 204 No Content path where Salesforce tells us nothing about what it matched.
// Returns "" on any failure: the caller treats a missing ID as cosmetic, not as
// a reason to fail a write that already went through.
//
// The value is rendered against the field's real type, not guessed from its
// text: an External ID field is often Number-typed (an order number), and a
// quoted literal against a numeric field is a hard INVALID_FIELD — which here
// would silently cost the flow the record ID it needs downstream.
func lookupID(instanceURL, token, extField, extValue string) string {
	soql, err := salesforce.BuildQueryTyped(
		instanceURL, token,
		"Opportunity",
		"Id",
		[]salesforce.Condition{{Field: extField, Operator: "=", Value: extValue}},
		false, "", 1, true,
	)
	if err != nil {
		return ""
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil || record == nil {
		return ""
	}
	return salesforce.StringifyID(record["Id"])
}

package crm_salesforce_opportunity_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Opportunity"
	Description  = "Create a deal in Salesforce with its value, stage and expected close date. Use it when an enquiry turns into something worth quoting for."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Opportunity Name", Placeholder: "Acme Ltd - 50 seat renewal", Required: true},
	{Name: "close_date", Type: core.ConnectionTypeDateTime, Label: "Expected Close Date", Placeholder: "2026-09-30 (the date only - Salesforce ignores the time)", Required: true},
	{Name: "stage_name", Type: core.ConnectionTypeString, Label: "Stage", Placeholder: "Prospecting - must match a stage in your Salesforce sales process", Required: true},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Company (Account)", Placeholder: "0015f00000AbCdEAAV - the Salesforce account this deal belongs to"},
	{Name: "amount", Type: core.ConnectionTypeString, Label: "Amount", Placeholder: "12500.00 - the deal value in your org's currency"},
	{Name: "probability", Type: core.ConnectionTypeInteger, Label: "Probability (%)", Placeholder: "40 - leave blank to let Salesforce derive it from the stage"},
	{Name: "opportunity_type", Type: core.ConnectionTypeString, Label: "Opportunity Type", Placeholder: "New Business or Existing Business"},
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

	// Salesforce rejects an Opportunity without all three of these, and it does
	// so with a REQUIRED_FIELD_MISSING that names the API field (StageName), not
	// the label the operator saw. Checking here turns that into a message that
	// points at the input they actually left empty.
	name := salesforce.OptionalString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("the opportunity name is required — what the deal is called in Salesforce, e.g. \"Acme Ltd - 50 seat renewal\"")
	}
	if salesforce.OptionalString("close_date", inputs) == "" {
		return nil, fmt.Errorf("the expected close date is required — Salesforce will not create a deal without one, e.g. 2026-09-30")
	}
	stage := salesforce.OptionalString("stage_name", inputs)
	if stage == "" {
		return nil, fmt.Errorf("the stage is required — it must match one of the sales stages configured in your Salesforce org, e.g. Prospecting")
	}

	body := map[string]interface{}{
		"Name":      name,
		"StageName": stage,
	}
	// CloseDate is a Date field, not a DateTime. A full ISO timestamp from a
	// date-picker upstream is rejected outright, so trim it to YYYY-MM-DD.
	salesforce.SetDateIfPresent(body, inputs, "CloseDate", "close_date")

	// Every optional field goes through Set*IfPresent so an untouched input is
	// omitted from the payload rather than sent blank — Salesforce treats an
	// explicit empty value as "clear this field".
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
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")

	// Custom fields are the normal path, not an edge case: every Salesforce org
	// has them and none of them can be first-class inputs here.
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Opportunity", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	return salesforce.RecordResult(id, raw, fmt.Sprintf("Created opportunity %q at stage %q (%s)", name, stage, id)), nil
}

// numericInput reads a decimal input, treating an unparseable value as the
// configuration mistake it is rather than silently dropping the field.
//

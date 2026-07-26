package crm_salesforce_opportunity_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Opportunity"
	Description  = "Change a deal in Salesforce - move it to the next stage, revise the value, push the close date out. Anything you leave blank is left exactly as it was."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "opportunity_id", Type: core.ConnectionTypeString, Label: "Opportunity ID", Placeholder: "0065f00000AbCdEAAV - the deal to change", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Opportunity Name", Placeholder: "Acme Ltd - 50 seat renewal"},
	{Name: "stage_name", Type: core.ConnectionTypeString, Label: "Stage", Placeholder: "Closed Won - must match a stage in your Salesforce sales process"},
	{Name: "close_date", Type: core.ConnectionTypeDateTime, Label: "Expected Close Date", Placeholder: "2026-09-30 (the date only - Salesforce ignores the time)"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Company (Account)", Placeholder: "0015f00000AbCdEAAV - the Salesforce account this deal belongs to"},
	{Name: "amount", Type: core.ConnectionTypeString, Label: "Amount", Placeholder: "12500.00 - the deal value in your org's currency"},
	{Name: "probability", Type: core.ConnectionTypeInteger, Label: "Probability (%)", Placeholder: "40 - leave blank to let Salesforce derive it from the stage"},
	{Name: "opportunity_type", Type: core.ConnectionTypeString, Label: "Opportunity Type", Placeholder: "New Business or Existing Business"},
	{Name: "lead_source", Type: core.ConnectionTypeString, Label: "Lead Source", Placeholder: "Web, Partner, Trade Show - must match your org's Lead Source list"},
	{Name: "next_step", Type: core.ConnectionTypeString, Label: "Next Step", Placeholder: "Send the proposal by Friday"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Background on the deal, visible to everyone on the account"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign", Placeholder: "7015f000000AbCdAAK - the campaign that sourced this deal"},
	{Name: "pricebook_id", Type: core.ConnectionTypeString, Label: "Price Book", Placeholder: "01s5f000004AbCdAAK - cannot be changed once the deal has products"},
	{Name: "forecast_category", Type: core.ConnectionTypeString, Label: "Forecast Category", Placeholder: "Pipeline, Best Case, Commit, Closed - normally derived from the stage"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "0055f00000AbCdEAAV - the salesperson who owns the deal"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "0125f000000AbCdAAK - only if your org uses record types"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other Salesforce field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Opportunity ID"},
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

	id := salesforce.OptionalString("opportunity_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// Every field is optional and every one goes through Set*IfPresent: an
	// update that posted all its blank inputs would clear the operator's data,
	// which on a live deal means wiping the amount, the owner and the next step
	// because they only wanted to move the stage.
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
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change on the opportunity")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Opportunity", id, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers an update with 204 No Content — there is no updated
	// record to return. Echo back what was actually applied (plus the ID) so the
	// next node has something to work with and the execution view shows what
	// changed. Use the Get Opportunity action if the full record is needed.
	changed := salesforce.SortedKeys(body)
	record := make(map[string]interface{}, len(body)+1)
	for k, v := range body {
		record[k] = v
	}
	record["Id"] = id

	return salesforce.RecordResult(id, record, fmt.Sprintf("Updated opportunity %s — changed %s", id, strings.Join(changed, ", "))), nil
}

// numericInput reads a decimal input, treating an unparseable value as the
// configuration mistake it is rather than silently dropping the field.
//

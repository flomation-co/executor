package crm_salesforce_campaign_create

import (
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Campaign"
	Description  = "Set up a marketing campaign in Salesforce — a webinar, open day, trade show or email blast — so people can be added to it and its results tracked. Leave a box blank to let Salesforce use its own default."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "campaign_name", Type: core.ConnectionTypeString, Label: "Campaign Name", Placeholder: "Autumn Open Day 2026", Required: true},
	{Name: "campaign_type", Type: core.ConnectionTypeString, Label: "Campaign Type", Placeholder: "Webinar, Conference, Trade Show, Email, Advertisement, Direct Mail, Other"},
	{Name: "campaign_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Planned, In Progress, Completed or Aborted"},
	{Name: "is_active", Type: core.ConnectionTypeBoolean, Label: "Active", Placeholder: "Tick this — Salesforce creates campaigns inactive, and an inactive campaign does not appear when staff search for one"},
	{Name: "start_date", Type: core.ConnectionTypeDateTime, Label: "Start Date", Placeholder: "2026-09-14 — the day the campaign begins (any time of day is ignored)"},
	{Name: "end_date", Type: core.ConnectionTypeDateTime, Label: "End Date", Placeholder: "2026-09-16 — the day the campaign finishes"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "What this campaign is for, who it targets, anything the team should know"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Parent Campaign", Placeholder: "701... — the bigger campaign this one rolls up into"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "005... — the Salesforce user who owns this campaign (defaults to the connected user)"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "012... — only needed if your org uses campaign record types"},
	{Name: "budgeted_cost", Type: core.ConnectionTypeMoney, Label: "Budgeted Cost", Placeholder: "5000.00"},
	{Name: "actual_cost", Type: core.ConnectionTypeMoney, Label: "Actual Cost", Placeholder: "4750.00"},
	{Name: "expected_revenue", Type: core.ConnectionTypeMoney, Label: "Expected Revenue", Placeholder: "25000.00"},
	{Name: "expected_response", Type: core.ConnectionTypeString, Label: "Expected Response Rate (%)", Placeholder: "12.5 — the percentage of people you expect to respond"},
	{Name: "number_sent", Type: core.ConnectionTypeInteger, Label: "Number Sent", Placeholder: "How many invitations or emails went out"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"CampaignMemberRecordTypeId":"012...","Region__c":"North"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign ID"},
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

	name := salesforce.OptionalString("campaign_name", inputs)
	if name == "" {
		return nil, fmt.Errorf("Campaign Name is required — every Salesforce campaign has to be named")
	}

	// Lookup IDs are checked here rather than left to Salesforce: a mistyped ID
	// comes back from the API as INVALID_CROSS_REFERENCE_KEY, which tells the
	// operator nothing about which of the three boxes was wrong.
	for _, ref := range []struct{ input, label string }{
		{"parent_id", "Parent Campaign"},
		{"owner_id", "Owner"},
		{"record_type_id", "Record Type"},
	} {
		if v := salesforce.OptionalString(ref.input, inputs); v != "" {
			if err := salesforce.ValidateRecordID(v); err != nil {
				return nil, fmt.Errorf("%s: %w", ref.label, err)
			}
		}
	}

	body := map[string]interface{}{"Name": name}

	salesforce.SetIfPresent(body, inputs, "Type", "campaign_type")
	salesforce.SetIfPresent(body, inputs, "Status", "campaign_status")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "ParentId", "parent_id")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")
	// IsActive is deliberately tri-state. SetBoolIfSet omits the field when the
	// checkbox was never touched, so Salesforce applies its own default
	// (inactive) rather than this action quietly deciding for the org.
	salesforce.SetBoolIfSet(body, inputs, "IsActive", "is_active")
	// StartDate and EndDate are Date fields, not DateTime. Salesforce rejects a
	// full ISO timestamp on them outright, so the time half is trimmed.
	salesforce.SetDateIfPresent(body, inputs, "StartDate", "start_date")
	salesforce.SetDateIfPresent(body, inputs, "EndDate", "end_date")
	salesforce.SetIntIfPresent(body, inputs, "NumberSent", "number_sent")

	for _, num := range []struct{ field, input, label string }{
		{"BudgetedCost", "budgeted_cost", "Budgeted Cost"},
		{"ActualCost", "actual_cost", "Actual Cost"},
		{"ExpectedRevenue", "expected_revenue", "Expected Revenue"},
		{"ExpectedResponse", "expected_response", "Expected Response Rate"},
	} {
		if err := setNumberIfPresent(body, inputs, num.field, num.input, num.label); err != nil {
			return nil, err
		}
	}

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Campaign", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers a create with {id, success, errors} — never the record
	// itself — so the raw response is what lands in "result". The new ID is on
	// the "id" output, which is what the next step in a flow actually needs, and
	// re-reading the record just to fill "result" would spend a second call out
	// of the org's daily API allowance on every run.
	return salesforce.RecordResult(id, raw, fmt.Sprintf("Created campaign %q (%s)", name, id)), nil
}

// setNumberIfPresent adds an optional numeric field to the record body, failing
// loudly when the operator typed something that is not a number.
//
// The shared SetFloatIfPresent silently omits a value it cannot parse, which on
// a cost field means the campaign is created with a blank budget and nobody
// notices until the finance report is wrong. Currency symbols and thousands
// separators are rejected rather than stripped: "5,5" is five-point-five in
// half of Europe and five thousand five hundred in the other half, and guessing
// which would be worse than asking.
func setNumberIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName, label string) error {
	raw := salesforce.OptionalString(inputName, inputs)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fmt.Errorf("%s must be a plain number such as 5000 or 5000.50 — leave out currency symbols and thousands separators (got %q)", label, raw)
	}
	body[field] = v
	return nil
}

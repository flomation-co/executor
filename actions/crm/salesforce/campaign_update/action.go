package crm_salesforce_campaign_update

import (
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Campaign"
	Description  = "Change a Salesforce campaign after the event — mark it Completed, move its dates, or record what it actually cost. Boxes you leave blank are left exactly as they are, so you can change one thing without disturbing the rest."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Placeholder: "701... — copy it from the end of the campaign's web address", Required: true},
	{Name: "campaign_name", Type: core.ConnectionTypeString, Label: "Campaign Name", Placeholder: "Autumn Open Day 2026"},
	{Name: "campaign_type", Type: core.ConnectionTypeString, Label: "Campaign Type", Placeholder: "Webinar, Conference, Trade Show, Email, Advertisement, Direct Mail, Other"},
	{Name: "campaign_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Planned, In Progress, Completed or Aborted"},
	{Name: "is_active", Type: core.ConnectionTypeBoolean, Label: "Active", Placeholder: "Tick to keep the campaign active, untick to retire it"},
	{Name: "start_date", Type: core.ConnectionTypeDateTime, Label: "Start Date", Placeholder: "2026-09-14 — the day the campaign begins (any time of day is ignored)"},
	{Name: "end_date", Type: core.ConnectionTypeDateTime, Label: "End Date", Placeholder: "2026-09-16 — the day the campaign finishes"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "What this campaign is for, who it targets, anything the team should know"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Parent Campaign", Placeholder: "701... — the bigger campaign this one rolls up into"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "005... — the Salesforce user who owns this campaign"},
	{Name: "budgeted_cost", Type: core.ConnectionTypeMoney, Label: "Budgeted Cost", Placeholder: "5000.00"},
	{Name: "actual_cost", Type: core.ConnectionTypeMoney, Label: "Actual Cost", Placeholder: "4750.00 — what the campaign really cost once the invoices are in"},
	{Name: "expected_revenue", Type: core.ConnectionTypeMoney, Label: "Expected Revenue", Placeholder: "25000.00"},
	{Name: "expected_response", Type: core.ConnectionTypeString, Label: "Expected Response Rate (%)", Placeholder: "12.5 — the percentage of people you expect to respond"},
	{Name: "number_sent", Type: core.ConnectionTypeInteger, Label: "Number Sent", Placeholder: "How many invitations or emails went out"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"Region__c":"North","EndDate":null} — a null empties that field`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign ID"},
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

	campaignID := salesforce.OptionalString("campaign_id", inputs)
	if err := salesforce.ValidateRecordID(campaignID); err != nil {
		return nil, err
	}
	for _, ref := range []struct{ input, label string }{
		{"parent_id", "Parent Campaign"},
		{"owner_id", "Owner"},
	} {
		if v := salesforce.OptionalString(ref.input, inputs); v != "" {
			if err := salesforce.ValidateRecordID(v); err != nil {
				return nil, fmt.Errorf("%s: %w", ref.label, err)
			}
		}
	}

	// Every field is Set*IfPresent, never assigned unconditionally: an update
	// that sent all seventeen inputs would blank the sixteen the operator left
	// alone. Emptying a field on purpose is done by putting an explicit null in
	// Additional Fields, which is the one place a "clear this" instruction can
	// be told apart from "I didn't fill that in".
	body := map[string]interface{}{}

	salesforce.SetIfPresent(body, inputs, "Name", "campaign_name")
	salesforce.SetIfPresent(body, inputs, "Type", "campaign_type")
	salesforce.SetIfPresent(body, inputs, "Status", "campaign_status")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "ParentId", "parent_id")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetBoolIfSet(body, inputs, "IsActive", "is_active")
	// StartDate and EndDate are Date fields, not DateTime — a full ISO
	// timestamp is rejected, so the time half is trimmed off.
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
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to change — fill in at least one field to update on the campaign")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Campaign", campaignID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers a successful update with 204 No Content, so there is no
	// record to hand back. Return the ID the caller already gave us plus the
	// changes that were applied — an empty result would strand anything wired to
	// this step's output.
	applied := map[string]interface{}{"Id": campaignID}
	for k, v := range body {
		applied[k] = v
	}
	summary := fmt.Sprintf("Updated campaign %s (%s)", campaignID, strings.Join(salesforce.SortedKeys(body), ", "))
	return salesforce.RecordResult(campaignID, applied, summary), nil
}

// setNumberIfPresent adds an optional numeric field to the record body, failing
// loudly when the operator typed something that is not a number.
//
// The shared SetFloatIfPresent silently omits a value it cannot parse, which on
// an Actual Cost field means the update quietly does nothing and the campaign
// still reports the old figure. Currency symbols and thousands separators are
// rejected rather than stripped: "5,5" is five-point-five in half of Europe and
// five thousand five hundred in the other half, and guessing would be worse
// than asking.
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

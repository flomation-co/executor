package crm_salesforce_opportunity_add_note

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Note to Opportunity (Classic)"
	Description  = "Attach a Classic note to a deal. Orgs on Lightning Enhanced Notes should use the Files and Notes actions instead - a Classic note does not appear in the Lightning Notes list."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+comment"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "opportunity_id", Type: core.ConnectionTypeString, Label: "Opportunity ID", Placeholder: "0065f00000AbCdEAAV - the deal to attach the note to", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Note Title", Placeholder: "Call summary - 25 July", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Note", Placeholder: "What was agreed on the call (up to 32 KB of plain text)"},
	{Name: "is_private", Type: core.ConnectionTypeBoolean, Label: "Private (only the note owner can see it)"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Note Owner", Placeholder: "0055f00000AbCdEAAV - defaults to the connected Salesforce user"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the Note object"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Note ID"},
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

	opportunityID := salesforce.OptionalString("opportunity_id", inputs)
	if err := salesforce.ValidateRecordID(opportunityID); err != nil {
		return nil, err
	}
	title := salesforce.OptionalString("title", inputs)
	if title == "" {
		return nil, fmt.Errorf("the note title is required — it is what shows in the deal's Notes list, e.g. \"Call summary - 25 July\"")
	}

	// This writes to the Note object, not to Opportunity: ParentId is the link
	// back to the deal. Note is the CLASSIC notes object — an org running
	// Lightning Enhanced Notes stores notes as ContentNote instead, and a
	// Classic note written here will not appear in the related list the user is
	// looking at. Hence the "(Classic)" in the action name.
	body := map[string]interface{}{
		"ParentId": opportunityID,
		"Title":    title,
	}
	salesforce.SetIfPresent(body, inputs, "Body", "body")
	salesforce.SetBoolIfSet(body, inputs, "IsPrivate", "is_private")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Note", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	return salesforce.RecordResult(id, raw, fmt.Sprintf("Added note %q to opportunity %s (%s)", title, opportunityID, id)), nil
}

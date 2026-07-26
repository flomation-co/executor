package crm_salesforce_opportunity_line_item_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Remove Product from Opportunity"
	Description  = "Take a product line off a deal. Salesforce recalculates the deal's value straight away."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+xmark"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "line_item_id", Type: core.ConnectionTypeString, Label: "Product Line ID", Placeholder: "00k5f00000AbCdEAAV - from Get Opportunity Products", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product Line ID"},
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

	id := salesforce.OptionalString("line_item_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// Removing a product line is not idempotent: running it twice on the same ID
	// answers ENTITY_IS_DELETED. That is Salesforce's answer, not a wiring
	// mistake, so it goes out on the error port as data and a flow can branch
	// on it rather than dying.
	if err := salesforce.DeleteRecord(instanceURL, token, "OpportunityLineItem", id); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers 204 No Content, so the ID we were given is all there is
	// to return — and it is what a tidy-up branch downstream actually uses.
	record := map[string]interface{}{"Id": id, "deleted": true}
	return salesforce.RecordResult(id, record, fmt.Sprintf("Removed product line %s from its opportunity", id)), nil
}

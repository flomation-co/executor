package crm_salesforce_opportunity_contact_role_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Opportunity Contact"
	Description  = "Change what part a contact plays in a deal, or make them the primary contact. Anything you leave blank is left exactly as it was."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pencil"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "contact_role_id", Type: core.ConnectionTypeString, Label: "Contact Role ID", Placeholder: "00K5f00000AbCdEAAV - from Get Opportunity Contacts", Required: true},
	{Name: "role", Type: core.ConnectionTypeString, Label: "Role", Placeholder: "Decision Maker, Influencer, Economic Buyer, Evaluator..."},
	{Name: "is_primary", Type: core.ConnectionTypeBoolean, Label: "Primary Contact On The Deal"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact", Placeholder: "0035f00000AbCdEAAV - only if the wrong person was linked"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the contact role"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact Role ID"},
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

	id := salesforce.OptionalString("contact_role_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}
	if contactID := salesforce.OptionalString("contact_id", inputs); contactID != "" {
		if err := salesforce.ValidateRecordID(contactID); err != nil {
			return nil, err
		}
	}

	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Role", "role")
	// SetBoolIfSet rather than OptionalBool: an untouched checkbox must be
	// omitted, or every role update would quietly demote the deal's primary
	// contact to "not primary".
	salesforce.SetBoolIfSet(body, inputs, "IsPrimary", "is_primary")
	salesforce.SetIfPresent(body, inputs, "ContactId", "contact_id")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — set a role, tick the primary box, or pick a different contact")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "OpportunityContactRole", id, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers an update with 204 No Content, so echo what was applied
	// alongside the ID rather than returning an empty result.
	changed := salesforce.SortedKeys(body)
	record := make(map[string]interface{}, len(body)+1)
	for k, v := range body {
		record[k] = v
	}
	record["Id"] = id

	return salesforce.RecordResult(id, record, fmt.Sprintf("Updated contact role %s — changed %s", id, strings.Join(changed, ", "))), nil
}

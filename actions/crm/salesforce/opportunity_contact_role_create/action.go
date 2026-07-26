package crm_salesforce_opportunity_contact_role_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Contact to Opportunity"
	Description  = "Link a contact to a deal and say what part they play in it - decision maker, influencer, technical buyer. This is how a deal stays reportable and how anyone knows who to ring."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "opportunity_id", Type: core.ConnectionTypeString, Label: "Opportunity ID", Placeholder: "0065f00000AbCdEAAV - the deal", Required: true},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact", Placeholder: "0035f00000AbCdEAAV - the person involved in the deal", Required: true},
	{Name: "role", Type: core.ConnectionTypeString, Label: "Role", Placeholder: "Decision Maker, Influencer, Economic Buyer, Evaluator..."},
	{Name: "is_primary", Type: core.ConnectionTypeBoolean, Label: "Primary Contact On The Deal"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the contact role"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact Role ID"},
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
	contactID := salesforce.OptionalString("contact_id", inputs)
	if err := salesforce.ValidateRecordID(contactID); err != nil {
		return nil, err
	}

	// OpportunityContactRole is a junction record: it holds nothing but the two
	// IDs, the role and the primary flag. Role is left free text rather than a
	// fixed list because the picklist is customised in most orgs — the editor
	// fills it from the org's own describe.
	body := map[string]interface{}{
		"OpportunityId": opportunityID,
		"ContactId":     contactID,
	}
	salesforce.SetIfPresent(body, inputs, "Role", "role")
	// Salesforce allows exactly one primary contact per deal and demotes the
	// current one automatically, so ticking this is a swap, not a clash.
	salesforce.SetBoolIfSet(body, inputs, "IsPrimary", "is_primary")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "OpportunityContactRole", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Linked contact %s to opportunity %s (%s)", contactID, opportunityID, id)
	if role := salesforce.OptionalString("role", inputs); role != "" {
		summary = fmt.Sprintf("Linked contact %s to opportunity %s as %q (%s)", contactID, opportunityID, role, id)
	}
	return salesforce.RecordResult(id, raw, summary), nil
}

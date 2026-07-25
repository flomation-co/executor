package crm_salesforce_account_contact_relation_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Relate Contact to Account"
	Description  = "Link a contact to another company they work with, and say what their role is there — a consultant who advises three of your customers, or a buyer who also sits on a parent company's board. The contact keeps their own main account."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account", Placeholder: "Record ID of the company to relate the contact to, e.g. 0015f00000AbCdEAAV", Required: true},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact", Placeholder: "Record ID of the contact, e.g. 0035f00000XyZabAAF", Required: true},
	{Name: "roles", Type: core.ConnectionTypeString, Label: "Roles", Placeholder: "Decision Maker, Business User (comma-separated; must match roles set up in your org)"},
	{Name: "is_active", Type: core.ConnectionTypeBoolean, Label: "Active Relationship"},
	{Name: "start_date", Type: core.ConnectionTypeDateTime, Label: "Start Date"},
	{Name: "end_date", Type: core.ConnectionTypeDateTime, Label: "End Date"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"Custom_Field__c":"value"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Relationship ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Relationship"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	accountID := salesforce.OptionalString("account_id", inputs)
	if accountID == "" {
		return nil, fmt.Errorf("account_id is required — the company to relate the contact to, e.g. 0015f00000AbCdEAAV")
	}
	if err := salesforce.ValidateRecordID(accountID); err != nil {
		return nil, fmt.Errorf("Account — %w", err)
	}
	contactID := salesforce.OptionalString("contact_id", inputs)
	if contactID == "" {
		return nil, fmt.Errorf("contact_id is required — the contact to relate, e.g. 0035f00000XyZabAAF")
	}
	if err := salesforce.ValidateRecordID(contactID); err != nil {
		return nil, fmt.Errorf("Contact — %w", err)
	}

	body := map[string]interface{}{
		"AccountId": accountID,
		"ContactId": contactID,
	}
	if roles := normaliseRoles(salesforce.OptionalString("roles", inputs)); roles != "" {
		body["Roles"] = roles
	}
	salesforce.SetBoolIfSet(body, inputs, "IsActive", "is_active")
	// StartDate and EndDate are Date fields, not Date/Time: a full timestamp is
	// rejected outright, so the helper trims one down to the date part.
	salesforce.SetDateIfPresent(body, inputs, "StartDate", "start_date")
	salesforce.SetDateIfPresent(body, inputs, "EndDate", "end_date")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "AccountContactRelation", body)
	if err != nil {
		// Two provider outcomes are common enough to expect: the org has not
		// enabled Contacts to Multiple Accounts (INVALID_TYPE), or the contact
		// already belongs to this account directly — Salesforce maintains that
		// relationship itself and refuses a second, indirect one.
		return salesforce.ErrorResult(err.Error()), nil
	}
	return salesforce.RecordResult(id, raw, fmt.Sprintf("Related contact %s to account %s", contactID, accountID)), nil
}

// normaliseRoles converts the operator's role list into the semicolon-separated
// form Salesforce requires for a multi-select picklist.
//
// Operators type "Decision Maker, Influencer" out of habit. Sent verbatim,
// Salesforce stores that as ONE role literally named "Decision Maker,
// Influencer" on an unrestricted picklist — no error, no roles anyone can
// report on, and nothing to tell the operator it went wrong.
func normaliseRoles(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' })
	roles := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			roles = append(roles, p)
		}
	}
	return strings.Join(roles, ";")
}

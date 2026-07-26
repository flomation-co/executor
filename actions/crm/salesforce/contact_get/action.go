// Package crm_salesforce_contact_get reads one Contact by its Salesforce record
// ID. It is the lookup step in front of almost every contact flow: fetch the
// person, then decide what to do with their email, owner or account.
package crm_salesforce_contact_get

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Contact"
	Description  = "Fetch a single contact by its Salesforce record ID, with every field the connected user can see. Narrow it to specific fields if you only need a few."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "0035f00000XyzAbAAJ — from the contact's Salesforce URL", Required: true},

	// Optional projection. Left blank Salesforce returns the whole record, which
	// is what an operator expects from "get this contact" — but a flow pulling
	// thousands of records benefits from asking for less.
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "FirstName, LastName, Email — leave blank for every field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Contact"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	contactID, err := salesforce.RequiredString("contact_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("contact_id is required — the 15 or 18 character ID from the contact's Salesforce URL")
	}
	// Checking the ID shape locally turns a confusing server-side MALFORMED_ID
	// into an immediate message naming the input that is wrong.
	if err := salesforce.ValidateRecordID(contactID); err != nil {
		return nil, err
	}

	fields, err := validatedFields(inputs)
	if err != nil {
		return nil, err
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Contact", contactID, fields)
	if err != nil {
		// A deleted contact, or one the connected user cannot see, is a provider
		// answer rather than a broken flow — it belongs on the error port.
		return salesforce.ErrorResult(err.Error()), nil
	}

	return salesforce.RecordResult(contactID, record, fmt.Sprintf("Retrieved contact %s", describeContact(record, contactID))), nil
}

// validatedFields whitelist-checks the optional field list before it is put on
// the wire. Field names cannot be quoted or escaped, so validation is the only
// defence available for them — and a typo is a configuration mistake worth
// failing on rather than a Salesforce error to relay.
func validatedFields(inputs []*core.Connection) (string, error) {
	names := salesforce.SplitList(salesforce.OptionalString("fields", inputs))
	if len(names) == 0 {
		return "", nil
	}
	validated := make([]string, 0, len(names))
	for _, name := range names {
		field, err := salesforce.ValidateSOQLFieldName(name)
		if err != nil {
			return "", err
		}
		validated = append(validated, field)
	}
	return strings.Join(validated, ","), nil
}

// describeContact renders the person's name for the summary line, falling back
// to the record ID when the operator asked for a field list that excludes it.
func describeContact(record map[string]interface{}, id string) string {
	first, _ := record["FirstName"].(string)
	last, _ := record["LastName"].(string)
	name := strings.TrimSpace(first + " " + last)
	if name == "" {
		return id
	}
	return name + " (" + id + ")"
}

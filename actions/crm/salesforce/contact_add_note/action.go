// Package crm_salesforce_contact_add_note attaches a Classic Note to a Contact.
//
// This writes the legacy Note object, which is what n8n does and what parity
// demands — but it is worth being honest about: an org running Lightning
// Enhanced Notes shows ContentNote records in its Notes panel, and a Classic
// Note written here will not appear there. Hence the "(Classic)" in the action
// name, so nobody picks it by accident and then cannot find their note.
package crm_salesforce_contact_add_note

import (
	"fmt"
	"unicode/utf8"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Note to Contact (Classic)"
	Description  = "Attach a note to a contact using Salesforce's Classic Notes. If your org uses Lightning Enhanced Notes, notes added this way appear under Notes & Attachments rather than the Notes panel."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+comment"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// maxNoteBody is Salesforce's hard limit on the Classic Note body. Checking it
// here turns a rejected call into an immediate, specific message — a call-summary
// note pasted from a transcript is exactly the thing that overruns it.
const maxNoteBody = 32000

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "0035f00000XyzAbAAJ — from the contact's Salesforce URL", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Note Title", Placeholder: "Call summary — 25 July", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Note", Placeholder: "What was discussed (up to 32,000 characters)"},

	// A private note is visible only to its owner and to users with "Modify All
	// Data" — which is rarely what a shared front-desk automation wants, so it
	// is off unless deliberately ticked.
	{Name: "is_private", Type: core.ConnectionTypeBoolean, Label: "Private (only the owner can see it)"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Note Owner", Placeholder: "Salesforce user ID, e.g. 0055f000004XyzAAB — defaults to the connected user"},

	// Notes carry fewer fields than most objects, but an org can still add its
	// own, so the escape hatch is here for consistency with every other write.
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Custom_Field__c":"value"}`},
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

	contactID, err := salesforce.RequiredString("contact_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("contact_id is required — the 15 or 18 character ID from the contact's Salesforce URL")
	}
	if err := salesforce.ValidateRecordID(contactID); err != nil {
		return nil, err
	}
	title, err := salesforce.RequiredString("title", inputs)
	if err != nil {
		return nil, fmt.Errorf("title is required — Salesforce will not accept a note without one")
	}

	noteBody := salesforce.OptionalString("body", inputs)
	if utf8.RuneCountInString(noteBody) > maxNoteBody {
		return nil, fmt.Errorf("the note is %d characters — Salesforce Classic Notes hold at most %d, so shorten it or attach it as a file instead", utf8.RuneCountInString(noteBody), maxNoteBody)
	}

	// ParentId is what parents the note to the contact; the Note object itself
	// has no ContactId field.
	body := map[string]interface{}{"Title": title, "ParentId": contactID}
	salesforce.SetIfPresent(body, inputs, "Body", "body")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetBoolIfSet(body, inputs, "IsPrivate", "is_private")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Note", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	return salesforce.RecordResult(id, raw, fmt.Sprintf("Added note %q to contact %s", title, contactID)), nil
}

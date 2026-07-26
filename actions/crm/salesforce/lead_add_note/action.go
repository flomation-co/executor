// Package crm_salesforce_lead_add_note attaches a Classic Note to a Lead.
//
// "Classic" is not decoration. This writes the legacy Note object, which is what
// n8n does and is kept here for parity — but in an org with Enhanced Notes
// switched on (the default for years) a Note written this way does NOT appear in
// the Lightning "Notes" related list the user is looking at. The record exists,
// it is just somewhere they will not think to look.
//
// If you want a note the person on the phone can actually see, use the
// Salesforce note action that writes a ContentNote. This one is here so existing
// Classic orgs and n8n migrations keep working.
package crm_salesforce_lead_add_note

import (
	"fmt"
	"unicode/utf8"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Note to Lead (Classic)"
	Description  = "Attach a Classic note to a lead. Note: in orgs using the newer Lightning notes, a Classic note will not show in the Notes panel."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+comment"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// maxNoteBody is Salesforce's hard limit on the Classic Note Body field:
// 32,000 characters. Checking it here turns a STRING_TOO_LONG rejection —
// which does not say which field or by how much — into a message that does.
const maxNoteBody = 32000

// maxNoteTitle is the Classic Note Title limit (80 characters).
const maxNoteTitle = 80

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "lead_id", Type: core.ConnectionTypeString, Label: "Lead ID", Placeholder: "00Q5f000004XyzAEAS", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Note Title", Placeholder: "Call summary — 25 July", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Note", Placeholder: "What was discussed (up to 32,000 characters)"},

	// Private notes are visible only to their owner and to users above them in
	// the role hierarchy — worth knowing before ticking it on a shared inbox.
	{Name: "is_private", Type: core.ConnectionTypeBoolean, Label: "Private (only the owner can see it)"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Note Owner", Placeholder: "Salesforce user ID, e.g. 0055f000004XyzAAB (defaults to the connected user)"},

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

	leadID, err := salesforce.RequiredString("lead_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("lead_id is required — the Salesforce record ID of the lead the note belongs to, e.g. 00Q5f000004XyzAEAS")
	}
	if err := salesforce.ValidateRecordID(leadID); err != nil {
		return nil, err
	}

	title, err := salesforce.RequiredString("title", inputs)
	if err != nil {
		return nil, fmt.Errorf("title is required — Salesforce will not accept a note without one")
	}
	if n := utf8.RuneCountInString(title); n > maxNoteTitle {
		return nil, fmt.Errorf("title is %d characters — Salesforce allows %d on a note title", n, maxNoteTitle)
	}

	// The lead is the note's PARENT, not a field on it: one Note object serves
	// every record type in Salesforce and ParentId is what tells it where to
	// hang. Getting this wrong creates an orphaned note attached to nothing.
	body := map[string]interface{}{
		"Title":    title,
		"ParentId": leadID,
	}

	if noteBody := salesforce.OptionalString("body", inputs); noteBody != "" {
		if n := utf8.RuneCountInString(noteBody); n > maxNoteBody {
			return nil, fmt.Errorf("the note is %d characters — Salesforce caps a Classic note at %d. Shorten it, or attach it as a file instead", n, maxNoteBody)
		}
		body["Body"] = noteBody
	}

	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	// SetBoolIfSet rather than a truthiness test, so an explicit "not private"
	// is transmitted. n8n only ever sends this field when it is true, which
	// means a flow can make a note private but never make one public again.
	salesforce.SetBoolIfSet(body, inputs, "IsPrivate", "is_private")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Note", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	return salesforce.RecordResult(id, raw, fmt.Sprintf("Added note %q to lead %s", title, leadID)), nil
}

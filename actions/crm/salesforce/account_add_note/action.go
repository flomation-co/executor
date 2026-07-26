package crm_salesforce_account_add_note

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Note to Account (Classic)"
	Description  = "Attach a note to a Salesforce account — a call summary, a delivery instruction, anything the team should see on the record. This writes a Classic Note, which appears in the account's Notes & Attachments list."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+comment"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// Salesforce's own limits on the Note object. Checking them here turns a
// STRING_TOO_LONG from Salesforce into a message that names the field and the
// limit before the request is ever sent.
const (
	maxTitleLength = 80
	maxBodyLength  = 32000
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account", Placeholder: "Record ID of the account the note belongs to, e.g. 0015f00000AbCdEAAV", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "Call summary — 14 March (up to 80 characters)", Required: true},
	{Name: "note_body", Type: core.ConnectionTypeText, Label: "Note", Placeholder: "Spoke to Jane about the renewal; she will confirm numbers on Friday"},
	{Name: "is_private", Type: core.ConnectionTypeBoolean, Label: "Private (only the note's owner and admins can see it)"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "Salesforce user ID to own the note; defaults to the connected user"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"Custom_Field__c":"value"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Note ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Note"},
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
		return nil, fmt.Errorf("account_id is required — the record ID of the account to attach the note to, e.g. 0015f00000AbCdEAAV")
	}
	if err := salesforce.ValidateRecordID(accountID); err != nil {
		return nil, err
	}

	title := salesforce.OptionalString("title", inputs)
	if title == "" {
		return nil, fmt.Errorf("title is required — Salesforce shows it as the note's heading")
	}
	if len([]rune(title)) > maxTitleLength {
		return nil, fmt.Errorf("title is %d characters — Salesforce allows at most %d for a note title", len([]rune(title)), maxTitleLength)
	}
	noteBody := salesforce.OptionalString("note_body", inputs)
	if len([]rune(noteBody)) > maxBodyLength {
		return nil, fmt.Errorf("the note is %d characters — Salesforce allows at most %d in a note", len([]rune(noteBody)), maxBodyLength)
	}
	if ownerID := salesforce.OptionalString("owner_id", inputs); ownerID != "" {
		if err := salesforce.ValidateRecordID(ownerID); err != nil {
			return nil, fmt.Errorf("Owner — %w", err)
		}
	}

	// The note is a Note record in its own right; the only thing tying it to
	// the account is ParentId, which is why this writes to /sobjects/Note and
	// not to the account.
	body := map[string]interface{}{
		"Title":    title,
		"ParentId": accountID,
	}
	salesforce.SetIfPresent(body, inputs, "Body", "note_body")
	// n8n declares an Owner option on this operation but reads the wrong input
	// name, so the dropdown is silently ignored and every note ends up owned by
	// the authenticating user. Wired properly here.
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetBoolIfSet(body, inputs, "IsPrivate", "is_private")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Note", body)
	if err != nil {
		// Orgs that have switched to Enhanced Notes can have the Classic Note
		// object turned off entirely, which surfaces as an INVALID_TYPE that
		// CheckResponse already explains. It is a provider outcome either way.
		return salesforce.ErrorResult(err.Error()), nil
	}
	return salesforce.RecordResult(id, raw, fmt.Sprintf("Added note %q to account %s", title, accountID)), nil
}

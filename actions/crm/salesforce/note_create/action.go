// Package crm_salesforce_note_create adds an Enhanced Note (ContentNote) to a
// record.
//
// This exists because the obvious action is the wrong one. n8n's "add note"
// writes the Classic Note object, which in an org with Enhanced Notes turned on
// — effectively every Lightning org — does not appear in the Notes related list
// at all. The call succeeds, a real record ID comes back, and the call summary
// the receptionist logged is invisible to the person who picks up the phone
// next. ContentNote is what Lightning shows, so it is what this writes.
//
// Two Salesforce quirks are handled here rather than left to the operator. The
// note body is stored as base64-encoded HTML with &, < and > escaped, and the
// note is not attached to anything until a second ContentDocumentLink call
// files it against the record.
package crm_salesforce_note_create

import (
	"encoding/base64"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Note"
	Description  = "Log a note against any Salesforce record — a call summary, a handover note — so it appears in the Notes related list your team reads."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+file-pen"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// maxNoteBytes is Salesforce's ceiling on a note body once encoded. Checking it
// here turns a wall of text into a clear message instead of a server-side
// rejection the operator cannot interpret.
const maxNoteBytes = 128 * 1024

// noteEscaper escapes the three characters Salesforce requires be entities
// inside a ContentNote body. Nothing else is touched: over-escaping would show
// literal entity codes in the note, which is just as wrong as under-escaping.
var noteEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "title", Type: core.ConnectionTypeString, Label: "Note Title", Placeholder: "Call with Jane Smith — 25 July", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Note", Placeholder: "What was said, agreed or needs doing next"},

	// Off by default because the overwhelming case is plain text typed by a
	// person, and unescaped angle brackets in plain text would be silently
	// swallowed as markup.
	{Name: "body_is_html", Type: core.ConnectionTypeBoolean, Label: "My Note Is Already HTML"},

	// object never reaches Salesforce — the link is by record ID alone. It is
	// here so the editor can narrow the record picker to one object type.
	{Name: "object", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Contact, Account, Case… — only used to help you pick the record"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Attach To Record", Placeholder: "The record the note belongs to, e.g. 0035f00000XyzAAB", Required: true},

	{
		Name:        "share_type",
		Type:        core.ConnectionTypeString,
		Label:       "Permission",
		Placeholder: "View Only is the safe default",
		Options: []core.ConnectionOption{
			{Name: "View Only", Value: "V"},
			{Name: "View and Edit", Value: "C"},
			{Name: "Inherit From The Record", Value: "I"},
		},
	},
	{
		Name:        "visibility",
		Type:        core.ConnectionTypeString,
		Label:       "Who Can See It",
		Placeholder: "All Users means everyone with access to the record, including community users",
		Options: []core.ConnectionOption{
			{Name: "All Users", Value: "AllUsers"},
			{Name: "Internal Users Only", Value: "InternalUsers"},
			{Name: "Shared Users", Value: "SharedUsers"},
		},
	},

	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"OwnerId":"0055f000004XyzAAB"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Note ID"},
	{Name: "link_id", Type: core.ConnectionTypeString, Label: "Share ID"},
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

	title, err := salesforce.RequiredString("title", inputs)
	if err != nil {
		return nil, fmt.Errorf("title is required — it is what the Notes related list shows")
	}
	parentID, err := salesforce.RequiredString("parent_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("parent_id is required — a note that is not attached to a record is invisible to everyone")
	}
	if err := salesforce.ValidateRecordID(parentID); err != nil {
		return nil, err
	}

	content := noteContent(salesforce.OptionalString("body", inputs), salesforce.OptionalBool("body_is_html", inputs))
	if len(content) > maxNoteBytes {
		return nil, fmt.Errorf("the note is too long — Salesforce accepts up to %d KB of note text", maxNoteBytes/1024)
	}

	body := map[string]interface{}{
		"Title": title,
		// Content is a base64 field on ContentNote, not plain text. Sending the
		// raw HTML gets a rejection that says nothing about encoding.
		"Content": base64.StdEncoding.EncodeToString([]byte(content)),
	}
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	noteID, raw, err := salesforce.CreateRecord(instanceURL, token, "ContentNote", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// A ContentNote's ID IS its ContentDocument ID, so the link can be made
	// straight away without another lookup.
	linkID, err := linkToRecord(instanceURL, token, noteID, parentID, inputs)
	if err != nil {
		// The note exists but is filed nowhere. Say so, and say where it is, so
		// the operator can attach it by hand rather than write it out again.
		return salesforce.ErrorResult(fmt.Sprintf("the note was created (%s) but could not be attached to %s: %s", noteID, parentID, err.Error())), nil
	}

	// Read the stored note back so the output carries real metadata. Purely
	// decorative: the note is written and linked by this point.
	record := raw
	if stored, err := salesforce.GetRecord(instanceURL, token, "ContentNote", noteID,
		"Id,Title,TextPreview,OwnerId,CreatedDate,LastModifiedDate"); err == nil && stored != nil {
		record = stored
	}

	out := salesforce.RecordResult(noteID, record, fmt.Sprintf("Added note %q to record %s", title, parentID))
	out["link_id"] = linkID
	return out, nil
}

// noteContent turns the operator's text into the HTML fragment ContentNote
// stores. Plain text has its markup characters escaped and its line breaks
// turned into <br> — without that, a multi-line call summary collapses into one
// unreadable paragraph, which is the single most common complaint about
// API-written notes.
func noteContent(body string, isHTML bool) string {
	if isHTML {
		return body
	}
	escaped := noteEscaper.Replace(body)
	// Normalise Windows line endings first, or each one becomes two breaks.
	escaped = strings.ReplaceAll(escaped, "\r\n", "\n")
	return strings.ReplaceAll(escaped, "\n", "<br>")
}

// linkToRecord files the note against the record. The defaults are the
// conservative ones: read-only, and no wider an audience than the record's own.
func linkToRecord(instanceURL, token, noteID, parentID string, inputs []*core.Connection) (string, error) {
	shareType := salesforce.OptionalString("share_type", inputs)
	if shareType == "" {
		shareType = "V"
	}
	visibility := salesforce.OptionalString("visibility", inputs)
	if visibility == "" {
		visibility = "AllUsers"
	}
	linkID, _, err := salesforce.CreateRecord(instanceURL, token, "ContentDocumentLink", map[string]interface{}{
		"ContentDocumentId": noteID,
		"LinkedEntityId":    parentID,
		"ShareType":         shareType,
		"Visibility":        visibility,
	})
	return linkID, err
}

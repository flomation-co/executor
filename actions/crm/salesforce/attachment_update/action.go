// Package crm_salesforce_attachment_update changes a Classic Attachment.
//
// The rule that matters here is Salesforce's, not ours: an OMITTED field is
// left alone, an EXPLICITLY BLANK one is cleared. Every input therefore goes
// through Set*IfPresent, so renaming an attachment does not also wipe its
// description and owner. An update that sent every blank box would quietly
// gut half the record, and nothing in the response would say so — Salesforce
// answers 204 No Content whether it changed one field or ten.
package crm_salesforce_attachment_update

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
	Name         = "Salesforce: Update Attachment (Classic)"
	Description  = "Rename a Classic attachment, change its description, owner or privacy, or replace the file itself. Anything you leave blank is left as it was."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pencil"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// maxAttachmentBytes is Salesforce's per-attachment ceiling. Base64 inflates
// the payload by a third on top of this, so the check is against the raw bytes.
const maxAttachmentBytes = 25 << 20 // 25 MB

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "attachment_id", Type: core.ConnectionTypeString, Label: "Attachment", Placeholder: "Attachment ID, e.g. 00P5f00000XyzAAB", Required: true},

	{Name: "file_name", Type: core.ConnectionTypeString, Label: "File Name", Placeholder: "contract-signed.pdf — leave blank to keep the current name"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Leave blank to keep the current description"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "Salesforce user ID, e.g. 0055f000004XyzAAB — leave blank to keep the current owner"},
	{Name: "is_private", Type: core.ConnectionTypeBoolean, Label: "Private (only the owner and admins can see it)"},

	{Name: "file", Type: core.ConnectionTypeString, Label: "Replace File With", Placeholder: "A file from an earlier step, or base64 content — leave blank to keep the current file"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type", Placeholder: "application/pdf — only needed when replacing the file with a different type"},

	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Custom_Field__c":"value"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Attachment"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	attachmentID, err := salesforce.RequiredString("attachment_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("attachment_id is required — the attachment you want to change")
	}
	if err := salesforce.ValidateRecordID(attachmentID); err != nil {
		return nil, err
	}

	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Name", "file_name")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "ContentType", "content_type")
	// SetBoolIfSet, not a truthiness check: without it there would be no way to
	// take an attachment back OFF private once it had been set.
	salesforce.SetBoolIfSet(body, inputs, "IsPrivate", "is_private")

	if err := setReplacementFile(flow, body, inputs); err != nil {
		return nil, err
	}
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Attachment", attachmentID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers an update with 204 No Content, so there is no record
	// in the response to return. Read it back so downstream steps get the
	// attachment as it now stands; if that read fails the update still
	// happened, so fall back to the ID rather than reporting a failure.
	record := map[string]interface{}{"Id": attachmentID}
	if stored, err := salesforce.GetRecord(instanceURL, token, "Attachment", attachmentID,
		"Id,Name,ContentType,BodyLength,Description,ParentId,OwnerId,IsPrivate,LastModifiedDate"); err == nil && stored != nil {
		record = stored
	}

	return salesforce.RecordResult(attachmentID, record,
		fmt.Sprintf("Updated attachment %s (%s)", attachmentID, strings.Join(salesforce.SortedKeys(body), ", "))), nil
}

// setReplacementFile swaps the attachment's contents when the operator supplied
// a new file, and does nothing at all when they did not — replacing the bytes
// is the one change that cannot be undone from the Recycle Bin, so it never
// happens by accident.
func setReplacementFile(flow *core.Flow, body map[string]interface{}, inputs []*core.Connection) error {
	source := salesforce.OptionalString("file", inputs)
	if source == "" {
		return nil
	}

	var raw []byte
	if core.IsFileRef(source) || core.IsBlobToken(source) {
		resolved, _, err := flow.ResolveToBytes(source)
		if err != nil {
			return fmt.Errorf("could not read the replacement file: %w", err)
		}
		raw = resolved
	} else {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(source))
		if err != nil {
			return fmt.Errorf("file must be a file from an earlier step, or base64-encoded content: %w", err)
		}
		raw = decoded
	}

	if len(raw) > maxAttachmentBytes {
		return fmt.Errorf("the replacement file is %d MB — a Classic attachment can hold up to %d MB. Use Upload File instead for anything larger", len(raw)>>20, maxAttachmentBytes>>20)
	}
	// Attachment.Body is a base64 field; handing it raw bytes stores a file
	// that is byte-for-byte wrong with no error anywhere.
	body["Body"] = base64.StdEncoding.EncodeToString(raw)
	return nil
}

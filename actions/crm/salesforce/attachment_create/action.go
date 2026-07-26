// Package crm_salesforce_attachment_create creates a Classic Attachment on a
// record.
//
// Attachment is Salesforce's legacy file object. It still works, some orgs and
// some managed packages still depend on it, and n8n's file support targets it
// exclusively — so it is here for parity and for those orgs. It is NOT the
// action most people want: a file written this way does not appear in the
// Lightning Files related list, so a contract uploaded here can be invisible to
// the person who needs it. That is why the name says "(Classic)" and why
// file_upload exists.
//
// The bytes are base64-encoded here rather than being taken as base64 from the
// operator, because everything upstream in Flomation emits a file reference or
// a blob token, and asking a receptionist to encode a PDF by hand is not a real
// option.
package crm_salesforce_attachment_create

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Attachment (Classic)"
	Description  = "Attach a file to a record using Salesforce's older Attachment object. For most orgs Upload File is the better choice — Classic attachments do not show in the Lightning Files list."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+paperclip"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// maxAttachmentBytes is Salesforce's per-attachment ceiling. Base64 inflates
// the payload by a third on top of this, so the check is against the raw bytes
// and the message points at the modern route for anything bigger.
const maxAttachmentBytes = 25 << 20 // 25 MB

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},

	// object never reaches Salesforce — the attachment is filed by record ID
	// alone. It is here so the editor can narrow the record picker.
	{Name: "object", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Account, Contact, Case… — only used to help you pick the record"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Attach To Record", Placeholder: "The record the file belongs to, e.g. 0015f00000XyzAAB", Required: true},

	{Name: "file_name", Type: core.ConnectionTypeString, Label: "File Name", Placeholder: "contract.pdf — include the extension", Required: true},
	{Name: "file", Type: core.ConnectionTypeString, Label: "File", Placeholder: "The file from an earlier step, or base64 content", Required: true},

	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type", Placeholder: "application/pdf — blank lets Salesforce work it out from the name"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "What this file is, for whoever finds it later"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "Salesforce user ID, e.g. 0055f000004XyzAAB — blank means you"},
	{Name: "is_private", Type: core.ConnectionTypeBoolean, Label: "Private (only the owner and admins can see it)"},

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

	parentID, err := salesforce.RequiredString("parent_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("parent_id is required — a Classic attachment always belongs to a record")
	}
	if err := salesforce.ValidateRecordID(parentID); err != nil {
		return nil, err
	}
	fileName, err := salesforce.RequiredString("file_name", inputs)
	if err != nil {
		return nil, fmt.Errorf("file_name is required — Salesforce shows it as the attachment's name")
	}
	source, err := salesforce.RequiredString("file", inputs)
	if err != nil {
		return nil, fmt.Errorf("file is required — wire in a file from an earlier step, or paste base64 content")
	}

	raw, err := resolveBytes(flow, source)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxAttachmentBytes {
		return nil, fmt.Errorf("the file is %d MB — a Classic attachment can hold up to %d MB. Use Upload File instead for anything larger", len(raw)>>20, maxAttachmentBytes>>20)
	}

	body := map[string]interface{}{
		"ParentId": parentID,
		"Name":     fileName,
		// Attachment.Body is a base64 field. Handing it the raw bytes produces
		// a stored file that is byte-for-byte wrong with no error anywhere.
		"Body": base64.StdEncoding.EncodeToString(raw),
	}
	if contentType := resolveContentType(salesforce.OptionalString("content_type", inputs), fileName); contentType != "" {
		body["ContentType"] = contentType
	}
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	// SetBoolIfSet, not a truthiness check, so an explicit "not private" is
	// transmitted rather than silently dropped.
	salesforce.SetBoolIfSet(body, inputs, "IsPrivate", "is_private")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, rawResp, err := salesforce.CreateRecord(instanceURL, token, "Attachment", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Read the stored record back so the output carries the real metadata
	// (length, content type, owner) rather than the create envelope. Purely
	// decorative — the attachment exists by this point either way.
	record := rawResp
	if stored, err := salesforce.GetRecord(instanceURL, token, "Attachment", id,
		"Id,Name,ContentType,BodyLength,Description,ParentId,OwnerId,IsPrivate,CreatedDate"); err == nil && stored != nil {
		record = stored
	}

	return salesforce.RecordResult(id, record, fmt.Sprintf("Attached %s to record %s (%d bytes)", fileName, parentID, len(raw))), nil
}

// resolveBytes accepts any of the three shapes a file arrives in: a workspace
// reference or blob token from an upstream action, or base64 typed in by hand.
// A value that is none of those is a configuration mistake, so it fails hard
// rather than uploading whatever the string happened to contain.
func resolveBytes(flow *core.Flow, value string) ([]byte, error) {
	if core.IsFileRef(value) || core.IsBlobToken(value) {
		raw, _, err := flow.ResolveToBytes(value)
		if err != nil {
			return nil, fmt.Errorf("could not read the file: %w", err)
		}
		return raw, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("file must be a file from an earlier step, or base64-encoded content: %w", err)
	}
	return raw, nil
}

// resolveContentType prefers the operator's value and otherwise leaves the
// field off entirely. Guessing from the extension is left to Salesforce, which
// already does it and knows its own defaults; sending a wrong type is worse
// than sending none, because the browser then refuses to preview the file.
func resolveContentType(explicit, fileName string) string {
	if explicit != "" {
		return explicit
	}
	if filepath.Ext(fileName) == "" {
		// No extension and no explicit type: octet-stream at least tells the
		// browser to download rather than try to render it as text.
		return "application/octet-stream"
	}
	return ""
}

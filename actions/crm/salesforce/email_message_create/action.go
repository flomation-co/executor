// Package crm_salesforce_email_message_create logs an email against a Case or
// another Salesforce record.
//
// This is the record-keeping half of email, not the sending half: it writes an
// EmailMessage row so that correspondence which happened somewhere else — a
// shared inbox, a helpdesk, an SMS gateway pretending to be email — shows up on
// the record in Salesforce. The person who picks up the phone next then sees
// the whole thread instead of asking the customer to repeat themselves.
//
// Two EmailMessage quirks drive the shape of this action:
//
//   - ParentId is a lookup to CASE ONLY, despite the generic-sounding name.
//     Anything else the email is about goes on RelatedToId, which is
//     polymorphic. Putting an Opportunity ID in ParentId fails with an
//     unhelpful cross-reference error, so the two are separate inputs here.
//   - Status is a REQUIRED picklist whose values are numeric strings ("0" New
//     … "5" Draft), not the words shown in the Salesforce UI. An operator will
//     never guess that, so it is a dropdown, and it is defaulted by direction
//     when left alone.
package crm_salesforce_email_message_create

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Log Email"
	Description  = "Record an email sent or received against a Case or another Salesforce record, so the whole thread is visible on the record."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+comment"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Case", Placeholder: "5005f00000AbcDEAA — the Case this email belongs to"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Related To Object", Placeholder: "Opportunity — narrows the Related To Record picker"},
	{Name: "related_to_id", Type: core.ConnectionTypeString, Label: "Related To Record", Placeholder: "0065f00000AbcDEAA — an Account, Opportunity or other record the email is about"},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Re: Your order #1042"},
	{Name: "text_body", Type: core.ConnectionTypeText, Label: "Message", Placeholder: "The plain text of the email"},
	{Name: "html_body", Type: core.ConnectionTypeText, Label: "Message (HTML)", Placeholder: "<p>The HTML version of the email</p>"},
	{Name: "from_address", Type: core.ConnectionTypeString, Label: "From Address", Placeholder: "jane@acme.com"},
	{Name: "from_name", Type: core.ConnectionTypeString, Label: "From Name", Placeholder: "Jane Bloggs"},
	{Name: "to_address", Type: core.ConnectionTypeString, Label: "To Address", Placeholder: "support@mycompany.com; sales@mycompany.com"},
	{Name: "cc_address", Type: core.ConnectionTypeString, Label: "CC Address", Placeholder: "manager@mycompany.com"},
	{Name: "bcc_address", Type: core.ConnectionTypeString, Label: "BCC Address", Placeholder: "archive@mycompany.com"},
	{Name: "incoming", Type: core.ConnectionTypeBoolean, Label: "This Email Came In (rather than went out)"},
	{
		Name:        "status",
		Type:        core.ConnectionTypeString,
		Label:       "Status",
		Placeholder: "Defaults to New for incoming email and Sent for outgoing",
		Options: []core.ConnectionOption{
			{Name: "New", Value: "0"},
			{Name: "Read", Value: "1"},
			{Name: "Replied", Value: "2"},
			{Name: "Sent", Value: "3"},
			{Name: "Forwarded", Value: "4"},
			{Name: "Draft", Value: "5"},
		},
	},
	{Name: "message_date", Type: core.ConnectionTypeDateTime, Label: "Date Sent or Received", Placeholder: "Defaults to now if left empty"},
	{Name: "is_externally_visible", Type: core.ConnectionTypeBoolean, Label: "Visible to Customers in the Portal"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"Headers\":\"...\",\"ThreadIdentifier\":\"ref:_00D...\"} — any other EmailMessage field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Email Message ID"},
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

	parentID := salesforce.OptionalString("parent_id", inputs)
	relatedToID := salesforce.OptionalString("related_to_id", inputs)

	// An EmailMessage attached to nothing is invisible: it does not appear on
	// any record's timeline and nobody will ever find it again. Refusing to
	// write one is kinder than writing a row the operator cannot see.
	if parentID == "" && relatedToID == "" {
		return nil, fmt.Errorf("nothing to log this email against — set the Case, or a Related To Record")
	}
	if parentID != "" {
		if err := salesforce.ValidateRecordID(parentID); err != nil {
			return nil, fmt.Errorf("Case: %w", err)
		}
	}
	if relatedToID != "" {
		if err := salesforce.ValidateRecordID(relatedToID); err != nil {
			return nil, fmt.Errorf("Related To Record: %w", err)
		}
	}
	// The Related To Object only narrows the record picker in the editor; it is
	// never sent, because RelatedToId is polymorphic and Salesforce works the
	// type out from the ID prefix. Validate it anyway so a typo is caught here
	// rather than leaving a picker that silently returns nothing.
	if object := salesforce.OptionalString("object", inputs); object != "" {
		if _, err := salesforce.ValidateSOQLObjectName(object); err != nil {
			return nil, fmt.Errorf("Related To Object: %w", err)
		}
	}

	body := map[string]interface{}{}
	if parentID != "" {
		body["ParentId"] = parentID
	}
	if relatedToID != "" {
		body["RelatedToId"] = relatedToID
	}
	salesforce.SetIfPresent(body, inputs, "Subject", "subject")
	salesforce.SetIfPresent(body, inputs, "TextBody", "text_body")
	salesforce.SetIfPresent(body, inputs, "HtmlBody", "html_body")
	salesforce.SetIfPresent(body, inputs, "FromAddress", "from_address")
	salesforce.SetIfPresent(body, inputs, "FromName", "from_name")
	salesforce.SetIfPresent(body, inputs, "ToAddress", "to_address")
	salesforce.SetIfPresent(body, inputs, "CcAddress", "cc_address")
	salesforce.SetIfPresent(body, inputs, "BccAddress", "bcc_address")
	salesforce.SetIfPresent(body, inputs, "MessageDate", "message_date")
	salesforce.SetBoolIfSet(body, inputs, "Incoming", "incoming")
	salesforce.SetBoolIfSet(body, inputs, "IsExternallyVisible", "is_externally_visible")

	// Status is required by Salesforce and its values are opaque numbers. When
	// the operator leaves it alone, pick the one that matches the direction
	// they did set: a received email is New, a sent one is Sent.
	incoming := salesforce.OptionalBool("incoming", inputs)
	status := salesforce.OptionalString("status", inputs)
	if status == "" {
		status = statusSent
		if incoming {
			status = statusNew
		}
	}
	body["Status"] = status

	// MessageDate is what every "read the thread" view sorts on, including this
	// node's own Get Many Emails (ORDER BY MessageDate DESC). Salesforce does
	// NOT populate it — it is a plain nillable DateTime — and SOQL sorts nulls
	// LAST on a descending order, so an email logged without one lands at the
	// BOTTOM of the thread it was just added to, reading as the oldest message
	// in the conversation. Defaulting to now is what the input's own label
	// promises, and it is set before the merge below so an explicit
	// MessageDate in Additional Fields still wins.
	if salesforce.OptionalString("message_date", inputs) == "" {
		body["MessageDate"] = time.Now().UTC().Format(time.RFC3339)
	}

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "EmailMessage", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	direction := "outgoing"
	if incoming {
		direction = "incoming"
	}
	attachedTo := parentID
	if attachedTo == "" {
		attachedTo = relatedToID
	}
	summary := fmt.Sprintf("Logged an %s email (%s) against record %s", direction, statusLabel(status), attachedTo)
	return salesforce.RecordResult(id, raw, summary), nil
}

// EmailMessage.Status values. Salesforce stores them as numeric STRINGS, and
// the numbers are the only thing the API accepts — the words below are what the
// UI shows for each.
const (
	statusNew  = "0"
	statusSent = "3"
)

// statusLabel renders a Status code as the word a Salesforce user would
// recognise, so the run summary reads like the record does.
func statusLabel(status string) string {
	switch status {
	case "0":
		return "New"
	case "1":
		return "Read"
	case "2":
		return "Replied"
	case "3":
		return "Sent"
	case "4":
		return "Forwarded"
	case "5":
		return "Draft"
	}
	return "status " + status
}

// Package crm_salesforce_email_send sends an email out through Salesforce
// itself rather than through a separate mail provider.
//
// The point of sending from Salesforce (instead of wiring in Gmail or SendGrid)
// is that Salesforce owns the deliverability, the org-wide from-address, the
// email templates the marketing team already built, and — with Log Email on
// Send — the activity history on the record. A rep opening the Contact
// afterwards sees the email in the timeline, which is the whole reason the
// customer bought a CRM.
//
// There is no REST endpoint for "send an email" on the sObject API. The path is
// the standard invocable action emailSimple, which is the same action Flow's
// "Send Email" element runs, so anything an admin can configure in Flow is
// reachable here. Its envelope is unusual in two ways worth knowing before
// changing this file:
//
//   - The request is {"inputs":[{...}]} — a LIST of parameter sets, because one
//     invocable-action call can fan out. This action always sends exactly one.
//   - The response is a JSON ARRAY of per-input results, and a FAILED send can
//     still come back HTTP 200 with isSuccess:false and a populated errors
//     array. Checking the status code alone would report a silent non-delivery
//     as a success.
package crm_salesforce_email_send

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Send Email"
	Description  = "Send an email through Salesforce, optionally from an email template and logged against a record so it shows in the timeline."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+envelope"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "to_addresses", Type: core.ConnectionTypeString, Label: "Send To", Placeholder: "jane@acme.com, accounts@acme.com"},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Your order is on its way"},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Message", Placeholder: "Hi Jane, thanks for getting in touch..."},
	{Name: "html_body", Type: core.ConnectionTypeText, Label: "Message (HTML)", Placeholder: "<p>Hi Jane,</p> — use this instead of Message, not as well as"},
	{Name: "email_template_id", Type: core.ConnectionTypeString, Label: "Email Template", Placeholder: "00X5f000000AbcDEA2 — a template written by your Salesforce admin"},
	{Name: "recipient_id", Type: core.ConnectionTypeString, Label: "Recipient Record", Placeholder: "0035f00000AbcDEAA — the Contact, Lead or User being emailed"},
	{Name: "related_record_id", Type: core.ConnectionTypeString, Label: "Related Record", Placeholder: "0065f00000AbcDEAA — the record the email is about, and where it is logged"},
	{
		Name:        "sender_type",
		Type:        core.ConnectionTypeString,
		Label:       "Send From",
		Placeholder: "Who the email appears to come from",
		Options: []core.ConnectionOption{
			{Name: "The Connected User", Value: "CurrentUser"},
			{Name: "The Default Workflow User", Value: "DefaultWorkflowUser"},
			{Name: "An Org-Wide Email Address", Value: "OrgWideEmailAddress"},
		},
	},
	{Name: "org_wide_email_address_id", Type: core.ConnectionTypeString, Label: "Org-Wide Email Address", Placeholder: "noreply@acme.com, or the address record ID — must be verified in Salesforce"},
	{Name: "log_email_on_send", Type: core.ConnectionTypeBoolean, Label: "Log This Email on the Record"},
	{Name: "use_line_breaks", Type: core.ConnectionTypeBoolean, Label: "Keep Line Breaks in the Message"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"emailAddressesArray\":[\"jane@acme.com\"]} — any other Send Email input, including CC and BCC"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Related Record ID"},
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

	to := salesforce.OptionalString("to_addresses", inputs)
	subject := salesforce.OptionalString("subject", inputs)
	body := salesforce.OptionalString("body", inputs)
	htmlBody := salesforce.OptionalString("html_body", inputs)
	templateID := salesforce.OptionalString("email_template_id", inputs)
	recipientID := salesforce.OptionalString("recipient_id", inputs)
	relatedID := salesforce.OptionalString("related_record_id", inputs)

	// Additional Fields is parsed BEFORE the completeness checks below rather
	// than merged after them, because those checks have to be able to see it.
	// Not every emailSimple parameter has a first-class input — CC, BCC and the
	// emailAddressesArray form of the recipient list all arrive this way, which
	// is exactly what this action's own Additional Fields placeholder tells the
	// operator to do. Validating the first-class inputs alone would refuse a
	// correctly-configured node for "nothing to send to" without ever looking
	// at the value the operator supplied.
	extra := map[string]interface{}{}
	if err := salesforce.MergeAdditionalFields(extra, inputs); err != nil {
		return nil, err
	}

	// Salesforce needs somewhere to send it. Either a literal address list or a
	// recipient record (whose email address it looks up) will do — but with
	// neither, emailSimple fails server-side with a message that does not say
	// which of the two is missing.
	if to == "" && recipientID == "" && !supplied(extra, "emailAddresses", "emailAddressesArray", "recipientId") {
		return nil, fmt.Errorf("nothing to send to — set Send To with one or more email addresses, or pick a Recipient Record")
	}
	// Likewise it needs something to say. A template brings its own subject and
	// body, so both are only required when there is no template — checking the
	// pair here beats Salesforce's server-side complaint, which names the wire
	// parameter (emailSubject) rather than the field the operator can see.
	if templateID == "" && body == "" && htmlBody == "" && !supplied(extra, "emailBody", "sendRichBody", "emailTemplateId") {
		return nil, fmt.Errorf("the email has no content — write a Message, or choose an Email Template")
	}
	if templateID == "" && subject == "" && !supplied(extra, "emailSubject", "emailTemplateId") {
		return nil, fmt.Errorf("the email has no Subject — write one, or choose an Email Template that supplies its own")
	}
	// emailSimple accepts a plain-text body OR a rich-text body, never both.
	// Rejecting it here is clearer than Salesforce's own error, which reads as
	// a generic invalid-input complaint. Additional Fields OVERWRITES the
	// matching first-class input rather than adding to it, so a key is only a
	// second body when the input it overwrites was empty.
	bodies := 0
	if body != "" || supplied(extra, "emailBody") {
		bodies++
	}
	if htmlBody != "" || supplied(extra, "sendRichBody") {
		bodies++
	}
	if bodies > 1 {
		return nil, fmt.Errorf("set either Message or Message (HTML), not both — Salesforce sends one body, not two")
	}

	if recipientID != "" {
		if err := salesforce.ValidateRecordID(recipientID); err != nil {
			return nil, fmt.Errorf("Recipient Record: %w", err)
		}
	}
	if relatedID != "" {
		if err := salesforce.ValidateRecordID(relatedID); err != nil {
			return nil, fmt.Errorf("Related Record: %w", err)
		}
	}
	if templateID != "" {
		if err := salesforce.ValidateRecordID(templateID); err != nil {
			return nil, fmt.Errorf("Email Template: %w", err)
		}
	}

	// Logging the email needs a record to log it against. Salesforce silently
	// sends-but-does-not-log otherwise, which is the worst possible outcome:
	// the operator ticked the box and the timeline stays empty.
	logOnSend := salesforce.OptionalBool("log_email_on_send", inputs)
	if v, ok := extra["logEmailOnSend"].(bool); ok {
		logOnSend = v
	}
	if logOnSend && relatedID == "" && recipientID == "" && !supplied(extra, "relatedRecordId", "recipientId") {
		return nil, fmt.Errorf("Log This Email on the Record needs somewhere to log it — set a Related Record or a Recipient Record")
	}

	// The sender. An org-wide address is the usual choice for anything that
	// should look like it came from the company rather than from whoever's
	// token happens to be connected.
	senderType := salesforce.OptionalString("sender_type", inputs)
	senderAddress := salesforce.OptionalString("org_wide_email_address_id", inputs)
	if senderAddress != "" && !strings.Contains(senderAddress, "@") {
		// The live picker in the editor yields an OrgWideEmailAddress record ID
		// while somebody typing it in will type the address itself. Rejecting
		// the exact value the picker just supplied would be indefensible, so an
		// ID costs one extra query and works.
		if err := salesforce.ValidateRecordID(senderAddress); err != nil {
			return nil, fmt.Errorf("Org-Wide Email Address must be an email address or a Salesforce record ID: %w", err)
		}
		resolved, err := lookupOrgWideAddress(instanceURL, token, senderAddress)
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		senderAddress = resolved
	}
	if senderAddress != "" && senderType == "" {
		// Choosing an org-wide address is an unambiguous statement of intent;
		// making the operator also set the dropdown would just be a trap.
		senderType = "OrgWideEmailAddress"
	}
	if senderType == "OrgWideEmailAddress" && senderAddress == "" && !supplied(extra, "senderAddress") {
		return nil, fmt.Errorf("Send From is set to an org-wide email address, but no Org-Wide Email Address was given")
	}

	// The parameter set for the one invocable-action input. Field names are
	// emailSimple's own, not sObject field names.
	params := map[string]interface{}{}
	if to != "" {
		params["emailAddresses"] = to
	}
	if subject != "" {
		params["emailSubject"] = subject
	}
	if body != "" {
		params["emailBody"] = body
	}
	if htmlBody != "" {
		// The emailSimple invocable action has NO richTextBody parameter — its
		// live input list is emailBody + sendRichBody (a boolean), confirmed
		// against GET /actions/standard/emailSimple. Salesforce silently drops
		// an unknown key, so sending richTextBody produced a request with no
		// body at all and came back 200 / isSuccess:false with
		// REQUIRED_FIELD_MISSING "the Body parameter is required" — an
		// HTML-only send failed 100% of the time, and the error blamed a field
		// the operator had actually filled in.
		params["emailBody"] = htmlBody
		params["sendRichBody"] = true
	}
	if templateID != "" {
		params["emailTemplateId"] = templateID
	}
	if recipientID != "" {
		params["recipientId"] = recipientID
	}
	if relatedID != "" {
		params["relatedRecordId"] = relatedID
	}
	if senderType != "" {
		params["senderType"] = senderType
	}
	if senderAddress != "" {
		params["senderAddress"] = senderAddress
	}
	salesforce.SetBoolIfSet(params, inputs, "logEmailOnSend", "log_email_on_send")
	salesforce.SetBoolIfSet(params, inputs, "useLineBreaks", "use_line_breaks")

	// CC and BCC, attachments and anything else Salesforce adds to the Send
	// Email action in a future release ride in here rather than needing a code
	// change — the same escape hatch every other Salesforce action exposes.
	// Applied last so a key the operator wrote by hand beats the first-class
	// input it shadows, which is the convention across this node.
	for k, v := range extra {
		params[k] = v
	}

	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodPost, "/actions/standard/emailSimple", map[string]interface{}{
		"inputs": []interface{}{params},
	})
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// A 2xx is not proof of delivery: the per-input result carries its own
	// success flag, and a rejected send (unverified from-address, a template
	// the connected user cannot see) arrives as 200 + isSuccess:false.
	entry, err := firstActionResult(resp.Body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if failed, msg := actionFailed(entry); failed {
		return salesforce.ErrorResult("Salesforce did not send the email: " + msg), nil
	}

	summary := describeSend(to, recipientID, templateID, logOnSend, relatedID)
	// emailSimple returns no record ID of its own, so the related record — the
	// thing the operator will chain off — is the useful ID to surface.
	return salesforce.RecordResult(relatedID, entry, summary), nil
}

// supplied reports whether Additional Fields carries a usable value under any
// of the given emailSimple parameter names.
//
// "Usable" excludes a present-but-empty value, because the editor happily
// writes {"emailSubject":""} when somebody clears a field they had typed into,
// and treating that as "the subject is covered" would put the completeness
// checks straight back where they started.
func supplied(extra map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		v, ok := extra[key]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return true
			}
		case []interface{}:
			if len(t) > 0 {
				return true
			}
		default:
			return true
		}
	}
	return false
}

// lookupOrgWideAddress turns an OrgWideEmailAddress record ID into the literal
// address emailSimple wants, because senderAddress on the wire is an email
// address and not an ID.
func lookupOrgWideAddress(instanceURL, token, id string) (string, error) {
	soql, err := salesforce.BuildQuery(
		"OrgWideEmailAddress",
		"Id, Address, DisplayName",
		[]salesforce.Condition{{Field: "Id", Operator: "=", Value: id}},
		false, "", 1, true,
	)
	if err != nil {
		return "", err
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("no org-wide email address in Salesforce has the ID %q — check it is still set up and verified under Setup ▸ Organization-Wide Addresses", id)
	}
	address, _ := record["Address"].(string)
	if address == "" {
		return "", fmt.Errorf("the org-wide email address %q has no email address on it", id)
	}
	return address, nil
}

// firstActionResult pulls the single per-input result out of an invocable
// action's array response. The array is always as long as the inputs array we
// sent, which is one.
func firstActionResult(body []byte) (map[string]interface{}, error) {
	var results []map[string]interface{}
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("failed to parse the Salesforce Send Email response: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("Salesforce accepted the request but returned no result for the email — it may not have been sent")
	}
	return results[0], nil
}

// actionFailed reports whether a per-input result says the send failed, along
// with a readable reason. Invocable-action errors use statusCode where the rest
// of the REST API uses errorCode, so neither key can be assumed.
func actionFailed(entry map[string]interface{}) (bool, string) {
	// A missing flag is treated as success rather than an invented failure:
	// CheckResponse has already cleared the status code, and only a present-
	// and-false isSuccess is Salesforce actually saying no.
	sent, present := entry["isSuccess"].(bool)
	if !present || sent {
		return false, ""
	}
	msgs := []string{}
	for _, raw := range asSlice(entry["errors"]) {
		e, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		msg, _ := e["message"].(string)
		code, _ := e["statusCode"].(string)
		if code == "" {
			code, _ = e["errorCode"].(string)
		}
		switch {
		case msg != "" && code != "":
			msgs = append(msgs, msg+" ("+code+")")
		case msg != "":
			msgs = append(msgs, msg)
		case code != "":
			msgs = append(msgs, code)
		}
	}
	if len(msgs) == 0 {
		return true, "Salesforce gave no reason — check the from-address is verified and the connected user can send email"
	}
	return true, strings.Join(msgs, "; ")
}

// asSlice normalises a JSON value that may be a list, a single object or null
// into a list, so callers can range over it unconditionally.
func asSlice(v interface{}) []interface{} {
	switch t := v.(type) {
	case []interface{}:
		return t
	case map[string]interface{}:
		return []interface{}{t}
	}
	return nil
}

// describeSend renders the operator-facing summary. It names what was actually
// done rather than echoing the inputs back, so a glance at the run history
// answers "did that email go, and where did it land".
func describeSend(to, recipientID, templateID string, logOnSend bool, relatedID string) string {
	// Recipients can arrive through Additional Fields (emailAddressesArray, CC
	// and BCC have no first-class input), in which case neither of the two
	// values this summary is built from is set — say something true rather than
	// "sent an email to the recipient record " with nothing after it.
	target := to
	switch {
	case target != "":
	case recipientID != "":
		target = "the recipient record " + recipientID
	default:
		target = "the recipients set in Additional Fields"
	}
	summary := "Sent an email to " + target
	if templateID != "" {
		summary += " using template " + templateID
	}
	if logOnSend {
		where := relatedID
		if where == "" {
			where = recipientID
		}
		if where == "" {
			summary += " and logged it on the record"
		} else {
			summary += fmt.Sprintf(" and logged it on record %s", where)
		}
	}
	return summary
}

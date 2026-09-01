package marketing_sendgrid_mail_send

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"encoding/base64"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Send Email"
	Description  = "Send an email through SendGrid. Provide a verified From address and one or more recipients, then either write the subject and content directly or send a dynamic template. SendGrid accepts the message for delivery and returns its message ID."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+paper-plane"
	Date         = "09/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "SendGrid API key (SendGrid → Settings → API Keys), e.g. ${secrets.sendgrid_api}", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "Global", Value: ""},
			{Name: "EU (data residency)", Value: "eu"},
		},
		Placeholder: "Global unless your account uses an EU regional subuser — the EU host has no Marketing endpoints (contacts, lists, segments)",
	},
	{Name: "from_email", Type: core.ConnectionTypeString, Label: "From Email", Placeholder: "sender@yourdomain.com — must be a verified sender or an address on your authenticated domain", Required: true},
	{Name: "from_name", Type: core.ConnectionTypeString, Label: "From Name", Placeholder: "Jane at Acme"},
	{Name: "to", Type: core.ConnectionTypeString, Label: "To", Placeholder: "recipient@example.com — separate multiple addresses with commas", Required: true},
	{Name: "cc", Type: core.ConnectionTypeString, Label: "Cc", Placeholder: "Comma-separated — SendGrid rejects an address that appears more than once across To, Cc, and Bcc"},
	{Name: "bcc", Type: core.ConnectionTypeString, Label: "Bcc", Placeholder: "Comma-separated — SendGrid rejects an address that appears more than once across To, Cc, and Bcc"},
	{Name: "reply_to", Type: core.ConnectionTypeString, Label: "Reply To", Placeholder: "Comma-separated addresses replies should go to"},
	{Name: "use_template", Type: core.ConnectionTypeBoolean, Label: "Use Template", Placeholder: "Tick to send a dynamic template instead of writing the subject and content here"},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "The email subject line", Visible: &core.VisibleWhen{Field: "use_template", Values: []string{"", "false"}}},
	{
		Name:  "content_type",
		Type:  core.ConnectionTypeString,
		Label: "Content Type",
		Options: []core.ConnectionOption{
			{Name: "HTML", Value: "text/html"},
			{Name: "Plain Text", Value: "text/plain"},
		},
		Placeholder: "HTML unless you choose plain text",
		Visible:     &core.VisibleWhen{Field: "use_template", Values: []string{"", "false"}},
	},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Content", Placeholder: "The email body — HTML or plain text to match the Content Type", Visible: &core.VisibleWhen{Field: "use_template", Values: []string{"", "false"}}},
	{Name: "template_id", Type: core.ConnectionTypeString, Label: "Template", Placeholder: "The dynamic template to send (d-...) — see \"SendGrid: List Templates\"", Visible: &core.VisibleWhen{Field: "use_template", Values: []string{"true"}}},
	{Name: "dynamic_template_data", Type: core.ConnectionTypeObject, Label: "Template Data (JSON)", Placeholder: `{"first_name":"Jane"} — values for the template's {{handlebars}}`, Visible: &core.VisibleWhen{Field: "use_template", Values: []string{"true"}}},
	{Name: "send_at", Type: core.ConnectionTypeDateTime, Label: "Send At", Placeholder: "Schedule delivery up to 72 hours ahead, e.g. 2026-07-10T09:00:00Z"},
	{Name: "categories", Type: core.ConnectionTypeString, Label: "Categories", Placeholder: "Comma-separated category names (up to 10) for filtering your email stats"},
	{Name: "asm_group_id", Type: core.ConnectionTypeString, Label: "Unsubscribe Group", Placeholder: "The unsubscribe (ASM) group this email belongs to"},
	{Name: "sandbox_mode", Type: core.ConnectionTypeBoolean, Label: "Sandbox Mode", Placeholder: "Tick to validate the message without sending it (nothing is delivered or billed)"},
	{Name: "attachments", Type: core.ConnectionTypeObject, Label: "Attachments (JSON)", Placeholder: `[{"content":"<base64>","filename":"report.pdf","type":"application/pdf","disposition":"attachment"}]`},
	{Name: "custom_args", Type: core.ConnectionTypeObject, Label: "Custom Args (JSON)", Placeholder: `{"order_id":"12345"} — passed through to event webhook events`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `Any other SendGrid mail field, e.g. {"ip_pool_name":"marketing"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Message ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	fromEmail, err := sendgrid.RequiredString("from_email", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	toRaw, err := sendgrid.RequiredString("to", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	to := sendgrid.SplitCSV(toRaw)
	if to == nil {
		return sendgrid.ErrorResult("to is required"), nil
	}

	from := map[string]interface{}{"email": fromEmail}
	if name := sendgrid.OptionalString("from_name", inputs); name != "" {
		from["name"] = name
	}
	personalization := map[string]interface{}{"to": emailObjects(to)}
	if cc := sendgrid.SplitCSV(sendgrid.OptionalString("cc", inputs)); cc != nil {
		personalization["cc"] = emailObjects(cc)
	}
	if bcc := sendgrid.SplitCSV(sendgrid.OptionalString("bcc", inputs)); bcc != nil {
		personalization["bcc"] = emailObjects(bcc)
	}
	body := map[string]interface{}{
		"from":             from,
		"personalizations": []interface{}{personalization},
	}
	if replyTo := sendgrid.SplitCSV(sendgrid.OptionalString("reply_to", inputs)); replyTo != nil {
		body["reply_to_list"] = emailObjects(replyTo)
	}

	useTemplate, _ := sendgrid.OptionalBoolSet("use_template", inputs)
	if useTemplate {
		templateID := sendgrid.OptionalString("template_id", inputs)
		if templateID == "" {
			return sendgrid.ErrorResult("choose a Template when Use Template is ticked"), nil
		}
		body["template_id"] = templateID
		data, err := sendgrid.OptionalJSON("dynamic_template_data", inputs)
		if err != nil {
			return sendgrid.ErrorResult(err.Error()), nil
		}
		if data != nil {
			obj, ok := data.(map[string]interface{})
			if !ok {
				return sendgrid.ErrorResult(`dynamic_template_data must be a JSON object, e.g. {"first_name":"Jane"}`), nil
			}
			personalization["dynamic_template_data"] = obj
		}
	} else {
		subject := sendgrid.OptionalString("subject", inputs)
		if subject == "" {
			return sendgrid.ErrorResult("provide a Subject, or tick Use Template to send a dynamic template"), nil
		}
		content := sendgrid.OptionalString("content", inputs)
		if content == "" {
			return sendgrid.ErrorResult("provide Content, or tick Use Template to send a dynamic template"), nil
		}
		contentType := sendgrid.OptionalString("content_type", inputs)
		if contentType == "" {
			contentType = "text/html"
		}
		body["subject"] = subject
		body["content"] = []interface{}{map[string]interface{}{"type": contentType, "value": content}}
	}

	if v := sendgrid.OptionalString("send_at", inputs); v != "" {
		n, err := sendgrid.EpochSeconds(v)
		if err != nil {
			return sendgrid.ErrorResult(fmt.Sprintf("send_at: %s", err)), nil
		}
		personalization["send_at"] = n
	}
	sendgrid.SetCSVIfPresent(body, inputs, "categories", "categories")
	if v := sendgrid.OptionalString("asm_group_id", inputs); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return sendgrid.ErrorResult(fmt.Sprintf("asm_group_id must be a whole number (got %q)", v)), nil
		}
		body["asm"] = map[string]interface{}{"group_id": n}
	}
	sandbox, sandboxSet := sendgrid.OptionalBoolSet("sandbox_mode", inputs)
	if sandboxSet {
		body["mail_settings"] = map[string]interface{}{"sandbox_mode": map[string]interface{}{"enable": sandbox}}
	}
	attachments, err := sendgrid.OptionalJSON("attachments", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	if attachments != nil {
		arr, ok := attachments.([]interface{})
		if !ok {
			return sendgrid.ErrorResult(`attachments must be a JSON array, e.g. [{"content":"<base64>","filename":"report.pdf"}]`), nil
		}
		// Resolve any attachment whose "content" is a flo:file:/flo:blob: reference
		// (e.g. a large media action output) into the base64 SendGrid expects.
		for _, item := range arr {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			cs, _ := m["content"].(string)
			if core.IsFileRef(cs) || core.IsBlobToken(cs) {
				data, mimeType, rerr := flow.ResolveToBytes(cs)
				if rerr != nil {
					return sendgrid.ErrorResult("could not read an attachment: " + rerr.Error()), nil
				}
				m["content"] = base64.StdEncoding.EncodeToString(data)
				// SendGrid requires a filename on every attachment and shows
				// it to the recipient, so fill it in from the reference when
				// the caller wired in a file but did not name it.
				name, _ := m["filename"].(string)
				m["filename"] = core.UploadFilename(name, cs, mimeType, "attachment")
				if t, _ := m["type"].(string); t == "" && mimeType != "" {
					m["type"] = mimeType
				}
			}
		}
		body["attachments"] = arr
	}
	customArgs, err := sendgrid.OptionalJSON("custom_args", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	if customArgs != nil {
		obj, ok := customArgs.(map[string]interface{})
		if !ok {
			return sendgrid.ErrorResult(`custom_args must be a JSON object, e.g. {"order_id":"12345"}`), nil
		}
		personalization["custom_args"] = obj
	}
	if err := sendgrid.MergeAdditionalFields(body, inputs); err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	// SendGrid enforces MIME ordering in the content array (text/plain first,
	// then text/html, then anything else — 400 otherwise), so re-order after
	// additional_fields may have supplied its own parts.
	if parts, ok := body["content"].([]interface{}); ok {
		body["content"] = orderContent(parts)
	}

	_, headers, _, err := sendgrid.Do(auth, http.MethodPost, "/v3/mail/send", nil, body)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	messageID := headers.Get("X-Message-Id")
	result := map[string]interface{}{"message_id": messageID, "status": "accepted"}
	summary := fmt.Sprintf("Email to %s accepted for delivery", strings.Join(to, ", "))
	if sandboxSet && sandbox {
		summary = fmt.Sprintf("Email to %s validated in sandbox mode — nothing was sent", strings.Join(to, ", "))
	}
	return sendgrid.ResourceResult(messageID, result, summary), nil
}

// emailObjects turns a list of addresses into SendGrid's [{"email": ...}]
// shape used by to/cc/bcc and reply_to_list.
func emailObjects(addresses []string) []interface{} {
	out := make([]interface{}, 0, len(addresses))
	for _, a := range addresses {
		out = append(out, map[string]interface{}{"email": a})
	}
	return out
}

// orderContent stably sorts content parts into SendGrid's required MIME
// order: text/plain first, then text/html, then any other types.
func orderContent(parts []interface{}) []interface{} {
	rank := func(p interface{}) int {
		if m, ok := p.(map[string]interface{}); ok {
			switch m["type"] {
			case "text/plain":
				return 0
			case "text/html":
				return 1
			}
		}
		return 2
	}
	sort.SliceStable(parts, func(i, j int) bool { return rank(parts[i]) < rank(parts[j]) })
	return parts
}

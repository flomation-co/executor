// Package send_email sends an email via Microsoft Outlook.
package send_email

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Email"
	Description  = "Send an email via Outlook"
	Website      = "https://www.flomation.co"
	Icon         = "paper-plane"
	Date         = "03/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "to", Type: core.ConnectionTypeString, Label: "To (email addresses, comma-separated)", Required: true},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Body", Required: true},
	{Name: "cc", Type: core.ConnectionTypeString, Label: "CC (email addresses, comma-separated)"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type", Options: []core.ConnectionOption{
		{Name: "Text", Value: "Text"},
		{Name: "HTML", Value: "HTML"},
	}},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_OUTLOOK}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	to := microsoft.OptStr("to", inputs)
	if to == "" {
		return microsoft.ErrorResult("to is required")
	}
	subject := microsoft.OptStr("subject", inputs)
	if subject == "" {
		return microsoft.ErrorResult("subject is required")
	}
	bodyText := microsoft.OptStr("body", inputs)
	if bodyText == "" {
		return microsoft.ErrorResult("body is required")
	}

	cc := microsoft.OptStr("cc", inputs)
	contentType := microsoft.OptStr("content_type", inputs)
	if contentType == "" {
		contentType = "Text"
	}
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	tokens, err := microsoft.FetchTokens(flow, credential, "mail_send")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	// Build recipients
	toRecipients := buildRecipients(to)
	ccRecipients := buildRecipients(cc)

	message := map[string]interface{}{
		"subject": subject,
		"body": map[string]interface{}{
			"contentType": contentType,
			"content":     bodyText,
		},
		"toRecipients": toRecipients,
	}
	if len(ccRecipients) > 0 {
		message["ccRecipients"] = ccRecipients
	}

	payload := map[string]interface{}{
		"message": message,
	}

	reqBody, _ := json.Marshal(payload)
	endpoint := fmt.Sprintf("%s/me/sendMail", microsoft.GraphAPI)

	status, body, err := microsoft.DoRequest(flow, "POST", endpoint, token.AccessToken, reqBody)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(body)))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Email sent successfully to %s", to),
		"success":     true,
		"error":       "",
	}, nil
}

func buildRecipients(addresses string) []map[string]interface{} {
	if addresses == "" {
		return nil
	}
	var recipients []map[string]interface{}
	for _, addr := range strings.Split(addresses, ",") {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		recipients = append(recipients, map[string]interface{}{
			"emailAddress": map[string]interface{}{
				"address": addr,
			},
		})
	}
	return recipients
}

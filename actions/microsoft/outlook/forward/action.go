// Package forward forwards a Microsoft Outlook email to specified recipients.
package forward

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
	Name         = "Forward Email"
	Description  = "Forward an Outlook email to other recipients"
	Website      = "https://www.flomation.co"
	Icon         = "share"
	Date         = "03/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID", Required: true},
	{Name: "to", Type: core.ConnectionTypeString, Label: "To (email addresses, comma-separated)", Required: true},
	{Name: "comment", Type: core.ConnectionTypeString, Label: "Comment"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_OUTLOOK}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	messageID := microsoft.OptStr("message_id", inputs)
	if messageID == "" {
		return microsoft.ErrorResult("message_id is required")
	}
	to := microsoft.OptStr("to", inputs)
	if to == "" {
		return microsoft.ErrorResult("to is required")
	}

	comment := microsoft.OptStr("comment", inputs)
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
	var toRecipients []map[string]interface{}
	for _, addr := range strings.Split(to, ",") {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		toRecipients = append(toRecipients, map[string]interface{}{
			"emailAddress": map[string]interface{}{
				"address": addr,
			},
		})
	}

	payload := map[string]interface{}{
		"toRecipients": toRecipients,
	}
	if comment != "" {
		payload["comment"] = comment
	}

	reqBody, _ := json.Marshal(payload)
	endpoint := fmt.Sprintf("%s/me/messages/%s/forward", microsoft.GraphAPI, messageID)

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
		"tool_result": fmt.Sprintf("Email forwarded successfully to %s", to),
		"success":     true,
		"error":       "",
	}, nil
}

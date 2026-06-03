// Package read_email reads the full content of a Microsoft Outlook email.
package read_email

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Read Email"
	Description  = "Read the full content of an Outlook email message"
	Website      = "https://www.flomation.co"
	Icon         = "envelope"
	Date         = "03/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_OUTLOOK}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject"},
	{Name: "from", Type: core.ConnectionTypeString, Label: "From"},
	{Name: "to", Type: core.ConnectionTypeString, Label: "To"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Body"},
	{Name: "received_at", Type: core.ConnectionTypeString, Label: "Received At"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Full Email (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	messageID := microsoft.OptStr("message_id", inputs)
	if messageID == "" {
		return microsoft.ErrorResult("message_id is required")
	}

	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	tokens, err := microsoft.FetchTokens(flow, credential, "mail_read")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	endpoint := fmt.Sprintf("%s/me/messages/%s", microsoft.GraphAPI, messageID)

	status, body, err := microsoft.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(body)))
	}

	var msg struct {
		Subject          string `json:"subject"`
		ReceivedDateTime string `json:"receivedDateTime"`
		Body             struct {
			ContentType string `json:"contentType"`
			Content     string `json:"content"`
		} `json:"body"`
		From struct {
			EmailAddress struct {
				Name    string `json:"name"`
				Address string `json:"address"`
			} `json:"emailAddress"`
		} `json:"from"`
		ToRecipients []struct {
			EmailAddress struct {
				Name    string `json:"name"`
				Address string `json:"address"`
			} `json:"emailAddress"`
		} `json:"toRecipients"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse message: %v", err))
	}

	fromAddr := fmt.Sprintf("%s <%s>", msg.From.EmailAddress.Name, msg.From.EmailAddress.Address)

	var toAddrs []string
	for _, r := range msg.ToRecipients {
		toAddrs = append(toAddrs, fmt.Sprintf("%s <%s>", r.EmailAddress.Name, r.EmailAddress.Address))
	}
	toStr := ""
	if len(toAddrs) > 0 {
		toStr = toAddrs[0]
		if len(toAddrs) > 1 {
			toStr = fmt.Sprintf("%s (+%d more)", toAddrs[0], len(toAddrs)-1)
		}
	}

	return map[string]interface{}{
		"tool_result": msg.Body.Content,
		"subject":     msg.Subject,
		"from":        fromAddr,
		"to":          toStr,
		"body":        msg.Body.Content,
		"received_at": msg.ReceivedDateTime,
		"email":       string(body),
		"success":     true,
		"error":       "",
	}, nil
}

// Package send_chat_message sends a message to a Microsoft Teams chat.
package send_chat_message

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Chat Message"
	Description  = "Send a message to a Microsoft Teams chat conversation"
	Website      = "https://www.flomation.co"
	Icon         = "microsoft+paper-plane"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "chat_id", Type: core.ConnectionTypeString, Label: "Chat ID", Required: true},
	{Name: "message", Type: core.ConnectionTypeText, Label: "Message Content", Required: true},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type", Options: []core.ConnectionOption{
		{Name: "HTML", Value: "html"},
		{Name: "Text", Value: "text"},
	}},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_TEAMS}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	chatID := microsoft.OptStr("chat_id", inputs)
	if chatID == "" {
		return microsoft.ErrorResult("chat_id is required")
	}
	message := microsoft.OptStr("message", inputs)
	if message == "" {
		return microsoft.ErrorResult("message is required")
	}

	contentType := microsoft.OptStr("content_type", inputs)
	if contentType == "" {
		contentType = "html"
	}
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	tokens, err := microsoft.FetchTokens(flow, credential, "teams")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	payload := map[string]interface{}{
		"body": map[string]interface{}{
			"content":     message,
			"contentType": contentType,
		},
	}
	reqBody, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("%s/chats/%s/messages", microsoft.GraphAPI, chatID)

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

	var resp map[string]interface{}
	_ = json.Unmarshal(body, &resp)

	messageID := ""
	if id, ok := resp["id"].(string); ok {
		messageID = id
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Message sent successfully to chat %s", chatID),
		"message_id":  messageID,
		"success":     true,
		"error":       "",
	}, nil
}

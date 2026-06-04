// Package facebook_send_message sends a reply via Facebook Messenger.
package facebook_send_message

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"io"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
	fb "flomation.app/automate/executor/actions/social/facebook"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Facebook Send Message"
	Description  = "Send a message to a user via Facebook Messenger"
	Website      = "https://www.flomation.co"
	Icon         = "facebook+paper-plane"
	Date         = "24/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "Page Access Token", Placeholder: "${credentials.facebook}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeString, Label: "App Secret", Placeholder: "${secrets.facebook_app_secret}"},
	{Name: "recipient_id", Type: core.ConnectionTypeString, Label: "Recipient PSID", Required: true},
	{Name: "message_text", Type: core.ConnectionTypeText, Label: "Message Text", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "recipient_id", Type: core.ConnectionTypeString, Label: "Recipient ID"},
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	accessToken, err := fb.GetAccessToken(inputs)
	if err != nil {
		return errResult(err.Error())
	}
	appSecret := fb.GetAppSecret(inputs)

	recipientConn := core.FindConnection("recipient_id", inputs)
	if recipientConn == nil || recipientConn.String() == nil || *recipientConn.String() == "" {
		return errResult("recipient_id is required")
	}
	recipientID := *recipientConn.String()

	messageConn := core.FindConnection("message_text", inputs)
	if messageConn == nil || messageConn.String() == nil || *messageConn.String() == "" {
		return errResult("message_text is required")
	}
	messageText := *messageConn.String()

	// Don't send messages that contain unresolved variable references
	if strings.Contains(messageText, "${") && strings.Contains(messageText, "}") {
		return errResult(fmt.Sprintf("message_text contains unresolved variable references: %s", messageText))
	}

	// Build Messenger Send API payload (JSON, not form-encoded)
	payload := map[string]interface{}{
		"recipient":      map[string]string{"id": recipientID},
		"messaging_type": "RESPONSE",
		"message":        map[string]string{"text": messageText},
	}
	payloadBytes, _ := json.Marshal(payload)

	// Build URL with access token and optional appsecret_proof
	apiURL := fmt.Sprintf("https://graph.facebook.com/v19.0/me/messages?access_token=%s", accessToken)
	if appSecret != "" {
		mac := hmac.New(sha256.New, []byte(appSecret))
		mac.Write([]byte(accessToken))
		apiURL += "&appsecret_proof=" + hex.EncodeToString(mac.Sum(nil))
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewReader(payloadBytes)) // #nosec G107
	if err != nil {
		return errResult(fmt.Sprintf("Failed to send message: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))

	if resp.StatusCode != http.StatusOK {
		return errResult(fmt.Sprintf("Graph API error (%d): %s", resp.StatusCode, string(respBody)))
	}

	var result struct {
		RecipientID string `json:"recipient_id"`
		MessageID   string `json:"message_id"`
	}
	_ = json.Unmarshal(respBody, &result)

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Message sent to %s", recipientID),
		"recipient_id": result.RecipientID,
		"message_id":   result.MessageID,
		"success":      true,
		"error":        "",
	}, nil
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":  msg,
		"recipient_id": "",
		"message_id":   "",
		"success":      false,
		"error":        msg,
	}, nil
}

// Package facebook_get_conversation fetches Messenger conversation history
// between a Facebook Page and a user (by PSID).
package facebook_get_conversation

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	fb "flomation.app/automate/executor/actions/social/facebook"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Facebook Get Conversation"
	Description  = "Fetch Messenger conversation history between a Page and a user"
	Website      = "https://www.flomation.co"
	Icon         = "facebook"
	Date         = "24/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "Page Access Token", Placeholder: "${page_access_token}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeString, Label: "App Secret", Placeholder: "${secrets.FACEBOOK_APP_SECRET}"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User PSID", Placeholder: "${sender_id}", Required: true},
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Page ID", Placeholder: "${page_id}", Required: true},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Message Limit", Placeholder: "20"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "messages", Type: core.ConnectionTypeObject, Label: "Messages (array)"},
	{Name: "messages_json", Type: core.ConnectionTypeString, Label: "Messages (JSON)"},
	{Name: "message_count", Type: core.ConnectionTypeInteger, Label: "Message Count"},
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	MessageID string `json:"message_id,omitempty"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	accessToken, err := fb.GetAccessToken(inputs)
	if err != nil {
		return errResult(err.Error())
	}
	appSecret := fb.GetAppSecret(inputs)

	userID := fb.OptionalString("user_id", inputs)
	if userID == "" {
		return errResult("user_id (PSID) is required")
	}

	pageID := fb.OptionalString("page_id", inputs)
	if pageID == "" {
		return errResult("page_id is required")
	}

	msgLimit := fb.OptionalString("limit", inputs)
	if msgLimit == "" {
		msgLimit = "20"
	}

	// Step 1: Find the conversation between the page and the user
	params := url.Values{
		"user_id": {userID},
		"fields":  {"id"},
	}
	resp, err := fb.ExecuteAPI(accessToken, appSecret, "GET", fb.GraphAPIBase+"/"+pageID+"/conversations", params)
	if err != nil {
		return errResult(fmt.Sprintf("failed to find conversation: %v", err))
	}
	if err := fb.CheckResponse(resp); err != nil {
		return errResult(err.Error())
	}

	var convResult struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &convResult); err != nil {
		return errResult(fmt.Sprintf("failed to parse conversations: %v", err))
	}

	if len(convResult.Data) == 0 {
		return map[string]interface{}{
			"tool_result":     "No conversation found with this user",
			"messages":        []interface{}{},
			"messages_json":   "[]",
			"message_count":   int64(0),
			"conversation_id": "",
			"success":         true,
			"error":           "",
		}, nil
	}

	conversationID := convResult.Data[0].ID

	// Step 2: Fetch messages from the conversation
	msgParams := url.Values{
		"fields": {"message,from,created_time"},
		"limit":  {msgLimit},
	}
	msgResp, err := fb.ExecuteAPI(accessToken, appSecret, "GET", fb.GraphAPIBase+"/"+conversationID+"/messages", msgParams)
	if err != nil {
		return errResult(fmt.Sprintf("failed to fetch messages: %v", err))
	}
	if err := fb.CheckResponse(msgResp); err != nil {
		return errResult(err.Error())
	}

	var msgResult struct {
		Data []struct {
			ID          string `json:"id"`
			Message     string `json:"message"`
			CreatedTime string `json:"created_time"`
			From        struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"from"`
		} `json:"data"`
	}
	if err := json.Unmarshal(msgResp.Body, &msgResult); err != nil {
		return errResult(fmt.Sprintf("failed to parse messages: %v", err))
	}

	// Build message array in chronological order (API returns newest first)
	var messages []message
	for i := len(msgResult.Data) - 1; i >= 0; i-- {
		m := msgResult.Data[i]
		role := "user"
		if m.From.ID == pageID {
			role = "assistant"
		}

		// Parse and format timestamp
		ts := m.CreatedTime
		if t, err := time.Parse("2006-01-02T15:04:05-0700", ts); err == nil {
			ts = t.UTC().Format(time.RFC3339)
		}

		messages = append(messages, message{
			Role:      role,
			Content:   m.Message,
			Timestamp: ts,
			MessageID: m.ID,
		})
	}

	messagesJSON, _ := json.Marshal(messages)

	// Build a human-readable summary for tool_result
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("%d messages in conversation:\n", len(messages)))
	for _, m := range messages {
		summary.WriteString(fmt.Sprintf("[%s] %s: %s\n", m.Timestamp, m.Role, truncate(m.Content, 100)))
	}

	// Convert to []interface{} for the object output
	var messagesObj []interface{}
	for _, m := range messages {
		messagesObj = append(messagesObj, map[string]interface{}{
			"role":       m.Role,
			"content":    m.Content,
			"timestamp":  m.Timestamp,
			"message_id": m.MessageID,
		})
	}

	return map[string]interface{}{
		"tool_result":     summary.String(),
		"messages":        messagesObj,
		"messages_json":   string(messagesJSON),
		"message_count":   int64(len(messages)),
		"conversation_id": conversationID,
		"success":         true,
		"error":           "",
	}, nil
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":     msg,
		"messages":        []interface{}{},
		"messages_json":   "[]",
		"message_count":   int64(0),
		"conversation_id": "",
		"success":         false,
		"error":           msg,
	}, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

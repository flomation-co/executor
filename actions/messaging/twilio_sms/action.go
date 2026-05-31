// Package twilio_sms sends an SMS message via the Twilio REST API.
package twilio_sms

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Twilio SMS"
	Description  = "Send an SMS message via Twilio"
	Website      = "https://www.flomation.co"
	Icon         = "comment-sms"
	Date         = "29/05/2026"
	Type         = core.ActionTypeAction

	twilioAPIBase = "https://api.twilio.com/2010-04-01"
)

var Inputs = [...]core.Connection{
	{
		Name:        "account_sid",
		Type:        core.ConnectionTypeString,
		Label:       "Account SID",
		Placeholder: "ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		Required:    true,
	},
	{
		Name:        "auth_token",
		Type:        core.ConnectionTypeString,
		Label:       "Auth Token",
		Placeholder: "${secrets.TWILIO_AUTH_TOKEN}",
		Required:    true,
	},
	{
		Name:        "from",
		Type:        core.ConnectionTypeString,
		Label:       "From Number (E.164)",
		Placeholder: "+19876543210",
		Required:    true,
	},
	{
		Name:        "to",
		Type:        core.ConnectionTypeString,
		Label:       "To Number (E.164)",
		Placeholder: "${from}",
		Required:    true,
	},
	{
		Name:        "message",
		Type:        core.ConnectionTypeText,
		Label:       "Message Body",
		Placeholder: "Hello from Flomation!",
		Required:    true,
	},
	{
		Name:        "media_url",
		Type:        core.ConnectionTypeString,
		Label:       "Media URL (MMS)",
		Placeholder: "https://example.com/image.jpg",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "message_sid", Type: core.ConnectionTypeString, Label: "Message SID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	accountSID := optionalString("account_sid", inputs)
	authToken := optionalString("auth_token", inputs)
	from := optionalString("from", inputs)
	to := optionalString("to", inputs)
	message := optionalString("message", inputs)
	mediaURL := optionalString("media_url", inputs)

	if accountSID == "" || authToken == "" {
		return errResult("account_sid and auth_token are required")
	}
	if from == "" || to == "" {
		return errResult("from and to numbers are required")
	}

	if message == "" {
		return map[string]interface{}{
			"tool_result": "No message to send (empty body)",
			"message_sid": "",
			"success":     true,
			"error":       "",
		}, nil
	}

	// Build form data
	data := url.Values{}
	data.Set("From", from)
	data.Set("To", to)
	data.Set("Body", message)
	if mediaURL != "" {
		data.Set("MediaUrl", mediaURL)
	}

	endpoint := fmt.Sprintf("%s/Accounts/%s/Messages.json", twilioAPIBase, accountSID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return errResult(fmt.Sprintf("Failed to create request: %v", err))
	}
	req.SetBasicAuth(accountSID, authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to send SMS: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errResult(fmt.Sprintf("Twilio returned %d: %s", resp.StatusCode, string(respBody)))
	}

	// Extract message SID from JSON response
	sid := extractField(respBody, "sid")

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("SMS sent to %s (SID: %s)", to, sid),
		"message_sid": sid,
		"success":     true,
		"error":       "",
	}, nil
}

func optionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"message_sid": "",
		"success":     false,
		"error":       msg,
	}, nil
}

func extractField(data []byte, field string) string {
	needle := fmt.Sprintf(`"%s"`, field)
	s := string(data)
	idx := strings.Index(s, needle)
	if idx == -1 {
		return ""
	}
	// Find the value after the field name
	rest := s[idx+len(needle):]
	// Skip ": " or ":"
	rest = strings.TrimLeft(rest, ": ")
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:] // skip opening quote
	end := strings.Index(rest, `"`)
	if end == -1 {
		return ""
	}
	return rest[:end]
}
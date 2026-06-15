// Package make_call initiates an outbound Twilio voice call.
// It pre-registers a voice session with Launch (via the API proxy),
// then uses the Twilio Calls REST API with inline TwiML to connect
// the callee to a Media Stream. The returned session_id can be wired
// into a downstream voice_session loop node for bidirectional audio.
//
// This action works in any flow — it does not require an agent context.
package make_call

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Make Call"
	Description  = "Initiate an outbound Twilio voice call"
	Website      = "https://www.flomation.co"
	Icon         = "phone-arrow-up-right"
	Date         = "31/05/2026"
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
		Type:        core.ConnectionTypeSecret,
		Label:       "Auth Token",
		Placeholder: "${secrets.TWILIO_AUTH_TOKEN}",
		Required:    true,
	},
	{
		Name:        "from",
		Type:        core.ConnectionTypeSecret,
		Label:       "From Number (E.164)",
		Placeholder: "+19876543210",
		Required:    true,
	},
	{
		Name:        "to",
		Type:        core.ConnectionTypeString,
		Label:       "To Number (E.164)",
		Placeholder: "+441234567890",
		Required:    true,
	},
	{
		Name:        "timeout",
		Type:        core.ConnectionTypeInteger,
		Label:       "Ring Timeout (seconds)",
		Placeholder: "30",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "call_sid", Type: core.ConnectionTypeString, Label: "Call SID"},
	{Name: "session_id", Type: core.ConnectionTypeString, Label: "Session ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	accountSID := optStr("account_sid", inputs)
	authToken := optStr("auth_token", inputs)
	from := optStr("from", inputs)
	to := optStr("to", inputs)
	timeout := optStr("timeout", inputs)

	if accountSID == "" || authToken == "" {
		return errResult("account_sid and auth_token are required")
	}
	if from == "" || to == "" {
		return errResult("from and to numbers are required")
	}
	if timeout == "" {
		timeout = "30"
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return errResult("execution context with API URL is required")
	}

	sessionID := uuid.New().String()

	// Pre-register the voice session with Launch (via API proxy).
	// Launch returns the public WebSocket URL for the TwiML.
	wsURL, err := preRegisterSession(ctx, sessionID, from, to)
	if err != nil {
		return errResult(fmt.Sprintf("failed to pre-register voice session: %v", err))
	}

	// Build inline TwiML with <Connect><Stream>
	twiml := buildOutboundTwiML(wsURL, sessionID, from, to)

	// POST to Twilio Calls API
	endpoint := fmt.Sprintf("%s/Accounts/%s/Calls.json", twilioAPIBase, accountSID)

	formData := url.Values{}
	formData.Set("From", from)
	formData.Set("To", to)
	formData.Set("Twiml", twiml)
	formData.Set("Timeout", timeout)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return errResult(fmt.Sprintf("failed to create request: %v", err))
	}
	req.SetBasicAuth(accountSID, authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("failed to initiate call: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errResult(fmt.Sprintf("Twilio returned %d: %s", resp.StatusCode, string(respBody)))
	}

	callSID := extractField(respBody, "sid")

	log.WithFields(log.Fields{
		"session_id": sessionID,
		"call_sid":   callSID,
		"from":       from,
		"to":         to,
	}).Info("outbound call initiated")

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Call initiated to %s (SID: %s, Session: %s)", to, callSID, sessionID),
		"call_sid":    callSID,
		"session_id":  sessionID,
		"success":     true,
		"error":       "",
	}, nil
}

// preRegisterSession calls the API proxy to register the session with
// Launch before the Twilio call is placed. Returns the public WebSocket
// URL that should be used in the TwiML.
func preRegisterSession(ctx *core.ExecutionContext, sessionID, from, to string) (string, error) {
	registerURL := fmt.Sprintf("%s/api/v1/internal/voice-session/%s/register", ctx.APIURL, sessionID)

	// agent_id is optional — outbound calls work without an agent context
	agentID := ""
	if ctx.AgentID != "" {
		agentID = ctx.AgentID
	}

	body, _ := json.Marshal(map[string]string{
		"agent_id":      agentID,
		"caller_number": from,
		"twilio_number": to,
	})

	req, err := http.NewRequest(http.MethodPost, registerURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the response to get the WebSocket URL
	var result struct {
		WSURL string `json:"ws_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse registration response: %w", err)
	}
	if result.WSURL == "" {
		return "", fmt.Errorf("registration response missing ws_url")
	}

	log.WithFields(log.Fields{
		"session_id": sessionID,
		"ws_url":     result.WSURL,
	}).Debug("voice session pre-registered")

	return result.WSURL, nil
}

// buildOutboundTwiML creates inline TwiML for an outbound call that
// connects to a Media Stream.
func buildOutboundTwiML(wsURL, sessionID, from, to string) string {
	return fmt.Sprintf(
		`<Response><Connect><Stream url="%s">`+
			`<Parameter name="sessionId" value="%s"/>`+
			`<Parameter name="from" value="%s"/>`+
			`<Parameter name="to" value="%s"/>`+
			`</Stream></Connect></Response>`,
		xmlEscape(wsURL),
		xmlEscape(sessionID),
		xmlEscape(from),
		xmlEscape(to),
	)
}

// xmlEscape escapes special characters for safe use in XML attributes.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func optStr(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"call_sid":    "",
		"session_id":  "",
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
	rest := s[idx+len(needle):]
	rest = strings.TrimLeft(rest, ": ")
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:]
	end := strings.Index(rest, `"`)
	if end == -1 {
		return ""
	}
	return rest[:end]
}

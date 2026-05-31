// Package end_call terminates an active Twilio voice call by closing
// the voice session WebSocket. Can be used within a voice_session
// subgraph to hang up programmatically (e.g. after a farewell message).
package end_call

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	voicesession "flomation.app/automate/executor/actions/twilio/voice_session"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "End Call"
	Description  = "Terminate an active Twilio voice call"
	Website      = "https://www.flomation.co"
	Icon         = "phone"
	Date         = "30/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "session_id",
		Type:        core.ConnectionTypeString,
		Label:       "Voice Session ID",
		Placeholder: "${session_id}",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	sessionID := ""
	sessionIDConn := core.FindConnection("session_id", inputs)
	if sessionIDConn != nil && sessionIDConn.String() != nil {
		s := *sessionIDConn.String()
		// Skip unresolved variable references
		if s != "" && !strings.HasPrefix(s, "${") {
			sessionID = s
		}
	}

	// If session_id wasn't resolved from inputs, try the trigger data
	// which is available as a parent node output on the trigger node.
	if sessionID == "" {
		// Check all node results for a session_id (from trigger or voice_session)
		for _, result := range flow.GetAllNodeResults() {
			if sid, ok := result["session_id"].(string); ok && sid != "" {
				sessionID = sid
				break
			}
		}
	}

	if sessionID == "" {
		return map[string]interface{}{
			"tool_result": "No session_id available — cannot end call",
			"success":     false,
		}, nil
	}

	sess := voicesession.GetSession(sessionID)
	if sess == nil {
		return map[string]interface{}{
			"tool_result": "Call already ended or session not found",
			"success":     true,
		}, nil
	}

	voicesession.CleanupSessionByID(sessionID)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Call terminated (session %s)", sessionID),
		"success":     true,
	}, nil
}

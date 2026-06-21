// Package send_audio sends audio back to a Twilio voice call via the
// voice session WebSocket. Used within a voice_session loop subgraph
// to play TTS audio to the caller.
package send_audio

import (
	"encoding/base64"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	voicesession "flomation.app/automate/executor/actions/twilio/voice_session"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Audio"
	Description  = "Send audio back to a Twilio voice call"
	Website      = "https://www.flomation.co"
	Icon         = "phone+paper-plane"
	Date         = "29/05/2026"
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
	{
		Name:     "audio_base64",
		Type:     core.ConnectionTypeString,
		Label:    "Audio (base64, mulaw 8kHz)",
		Required: true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "audio_size_bytes", Type: core.ConnectionTypeInteger, Label: "Audio Size (bytes)"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	sessionID := optStr("session_id", inputs)
	if sessionID == "" {
		return errResult("session_id is required")
	}

	audioB64 := optStr("audio_base64", inputs)
	if audioB64 == "" || strings.HasPrefix(audioB64, "${") {
		return map[string]interface{}{
			"tool_result":      "No audio to send (empty or unresolved)",
			"success":          true,
			"audio_size_bytes": int64(0),
			"error":            "",
		}, nil
	}

	// Decode base64 audio
	audioData, err := base64.StdEncoding.DecodeString(audioB64)
	if err != nil {
		// Try URL-safe encoding
		audioData, err = base64.URLEncoding.DecodeString(audioB64)
		if err != nil {
			return errResult(fmt.Sprintf("failed to decode audio: %v", err))
		}
	}

	if len(audioData) == 0 {
		return map[string]interface{}{
			"tool_result":      "No audio data after decode",
			"success":          true,
			"audio_size_bytes": int64(0),
			"error":            "",
		}, nil
	}

	// Get the active voice session
	sess := voicesession.GetSession(sessionID)
	if sess == nil {
		return errResult("voice session not found or already closed")
	}

	// If the caller interrupted (barge-in), skip sending audio
	if sess.BargedIn() {
		return map[string]interface{}{
			"tool_result":      "Audio skipped — caller interrupted (barge-in)",
			"success":          true,
			"audio_size_bytes": int64(0),
			"error":            "",
		}, nil
	}

	// Send audio to the caller via WebSocket.
	// If the call has ended (broken pipe), treat as success — the audio
	// was produced but the caller hung up before it could be played.
	if err := sess.SendAudioToStream(audioData); err != nil {
		if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "closed") {
			return map[string]interface{}{
				"tool_result":      "Call ended before audio could be played",
				"success":          true,
				"audio_size_bytes": int64(len(audioData)),
				"error":            "",
			}, nil
		}
		return errResult(fmt.Sprintf("failed to send audio: %v", err))
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Audio sent to caller (%d bytes)", len(audioData)),
		"success":          true,
		"audio_size_bytes": int64(len(audioData)),
		"error":            "",
	}, nil
}

func optStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":      "Error: " + msg,
		"success":          false,
		"audio_size_bytes": int64(0),
		"error":            msg,
	}, nil
}

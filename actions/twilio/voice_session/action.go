// Package voice_session implements a long-running voice call loop node.
// It connects to the API's voice session WebSocket, receives mulaw audio
// from the caller, runs VAD to detect speech boundaries, and outputs
// the speech audio for subgraph processing (STT → AI → TTS).
//
// This action uses ActionTypeLoop — the executor calls Execute() on each
// iteration. On the first call it establishes the WebSocket connection.
// On each subsequent call it waits for the next speech segment.
package voice_session

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	core "flomation.app/automate/executor"
	twilio "flomation.app/automate/executor/actions/twilio"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Voice Session"
	Description  = "Hold a Twilio voice call open for multi-turn conversation"
	Website      = "https://www.flomation.co"
	Icon         = "phone-volume"
	Date         = "29/05/2026"
	Type         = core.ActionTypeLoop
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
		Name:        "max_turns",
		Type:        core.ConnectionTypeInteger,
		Label:       "Max Turns",
		Placeholder: "50",
	},
	{
		Name:        "max_duration_seconds",
		Type:        core.ConnectionTypeInteger,
		Label:       "Max Duration (seconds)",
		Placeholder: "3600",
	},
	{
		Name:        "silence_threshold",
		Type:        core.ConnectionTypeString,
		Label:       "Silence Threshold",
		Placeholder: "0.01",
	},
	{
		Name:        "silence_duration_ms",
		Type:        core.ConnectionTypeInteger,
		Label:       "Silence Duration (ms)",
		Placeholder: "1500",
	},
	{
		Name:        "min_speech_duration_ms",
		Type:        core.ConnectionTypeInteger,
		Label:       "Min Speech Duration (ms)",
		Placeholder: "300",
	},
	{
		Name:  "enable_barge_in",
		Type:  core.ConnectionTypeBoolean,
		Label: "Enable Barge-In",
	},
	{
		Name:        "greeting_audio_base64",
		Type:        core.ConnectionTypeString,
		Label:       "Greeting Audio (base64, mulaw 8kHz)",
		Placeholder: "Wire from a TTS node to play a greeting on call start",
	},
}

var Outputs = [...]core.Connection{
	{Name: "result", Type: core.ConnectionTypeBoolean, Label: "Continue Loop"},
	{Name: "voice_audio_base64", Type: core.ConnectionTypeString, Label: "Caller Speech (mulaw base64)"},
	{Name: "voice_audio_format", Type: core.ConnectionTypeString, Label: "Audio Format"},
	{Name: "turn_number", Type: core.ConnectionTypeInteger, Label: "Current Turn"},
	{Name: "current_index", Type: core.ConnectionTypeInteger, Label: "Loop Index"},
	{Name: "iterations", Type: core.ConnectionTypeInteger, Label: "Iteration Count"},
	{Name: "max_iterations", Type: core.ConnectionTypeInteger, Label: "Max Iterations"},
	{Name: "call_duration", Type: core.ConnectionTypeInteger, Label: "Call Duration (seconds)"},
	{Name: "total_turns", Type: core.ConnectionTypeInteger, Label: "Total Turns Completed"},
	{Name: "call_sid", Type: core.ConnectionTypeString, Label: "Call SID"},
	{Name: "stream_sid", Type: core.ConnectionTypeString, Label: "Stream SID"},
}

// activeSession holds the WebSocket connection and state for a voice session.
type activeSession struct {
	conn      *websocket.Conn
	streamSID string
	callSID   string
	vad       *twilio.VAD
	startedAt time.Time
	turnCount int
	maxTurns  int
	maxDur    time.Duration
	bargeIn   bool
	closed    bool
	mu        sync.Mutex
}

// sessions is a package-level registry of active voice sessions.
// Keyed by session_id. Entries are created on the first Execute() call
// and removed when the call ends or the flow completes.
var (
	sessions   = make(map[string]*activeSession)
	sessionsMu sync.Mutex
)

// wsMessage represents a Twilio Media Streams WebSocket message.
type wsMessage struct {
	Event     string `json:"event"`
	StreamSID string `json:"streamSid,omitempty"`
	Media     *struct {
		Payload   string `json:"payload"`
		Timestamp string `json:"timestamp"`
		Chunk     string `json:"chunk"`
	} `json:"media,omitempty"`
	Start *struct {
		StreamSID        string            `json:"streamSid"`
		CallSID          string            `json:"callSid"`
		CustomParameters map[string]string `json:"customParameters"`
		MediaFormat      struct {
			Encoding   string `json:"encoding"`
			SampleRate int    `json:"sampleRate"`
			Channels   int    `json:"channels"`
		} `json:"mediaFormat"`
	} `json:"start,omitempty"`
	Stop *struct {
		CallSID    string `json:"callSid"`
		AccountSID string `json:"accountSid"`
	} `json:"stop,omitempty"`
	Mark *struct {
		Name string `json:"name"`
	} `json:"mark,omitempty"`
	DTMF *struct {
		Digit string `json:"digit"`
		Track string `json:"track"`
	} `json:"dtmf,omitempty"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	sessionID := optStr("session_id", inputs)
	if sessionID == "" {
		return exitResult(0, 0, "", ""), fmt.Errorf("session_id is required")
	}

	maxTurns := optInt("max_turns", inputs, 50)
	maxDurSec := optInt("max_duration_seconds", inputs, 3600)
	silenceThresh := optFloat("silence_threshold", inputs, 0.01)
	silenceDurMs := optInt("silence_duration_ms", inputs, 1500)
	minSpeechMs := optInt("min_speech_duration_ms", inputs, 300)
	bargeIn := optStr("enable_barge_in", inputs) == "true"

	sessionsMu.Lock()
	sess, exists := sessions[sessionID]
	if !exists {
		// First iteration — establish WebSocket connection
		ctx := flow.GetContext()
		if ctx == nil || ctx.APIURL == "" {
			sessionsMu.Unlock()
			return exitResult(0, 0, "", ""), fmt.Errorf("API URL not available")
		}

		wsURL := buildWSURL(ctx.APIURL, sessionID)
		dialer := websocket.Dialer{
			TLSClientConfig: nil, // Will be set from transport
			HandshakeTimeout: 15 * time.Second,
		}

		// Use mTLS transport if available
		if transport, ok := ctx.InternalClient().Transport.(*http.Transport); ok && transport.TLSClientConfig != nil {
			dialer.TLSClientConfig = transport.TLSClientConfig
		}

		headers := http.Header{}
		conn, _, err := dialer.Dial(wsURL, headers)
		if err != nil {
			sessionsMu.Unlock()
			log.WithFields(log.Fields{
				"session_id": sessionID,
				"error":      err,
				"ws_url":     wsURL,
			}).Error("failed to connect to voice session WebSocket")
			return exitResult(0, 0, "", ""), fmt.Errorf("failed to connect to voice session: %w", err)
		}

		sess = &activeSession{
			conn:      conn,
			vad:       twilio.NewVADWithConfig(silenceThresh, silenceDurMs, minSpeechMs),
			startedAt: time.Now(),
			maxTurns:  maxTurns,
			maxDur:    time.Duration(maxDurSec) * time.Second,
			bargeIn:   bargeIn,
		}
		sessions[sessionID] = sess

		// Wait for the "start" event to get streamSid and callSid
		if err := waitForStart(sess); err != nil {
			sessionsMu.Unlock()
			cleanupSession(sessionID)
			return exitResult(0, 0, "", ""), fmt.Errorf("failed to receive start event: %w", err)
		}

		log.WithFields(log.Fields{
			"session_id": sessionID,
			"stream_sid": sess.streamSID,
			"call_sid":   sess.callSID,
		}).Info("voice session established")

		// Play greeting audio if provided (before first listen)
		greetingB64 := optStr("greeting_audio_base64", inputs)
		if greetingB64 != "" && !strings.HasPrefix(greetingB64, "${") {
			greetingData, err := base64.StdEncoding.DecodeString(greetingB64)
			if err == nil && len(greetingData) > 0 {
				if err := sess.SendAudioToStream(greetingData); err != nil {
					log.WithFields(log.Fields{
						"session_id": sessionID,
						"error":      err,
					}).Warn("failed to send greeting audio")
				} else {
					log.WithField("session_id", sessionID).Info("greeting audio sent")
				}
			}
		}
	}
	sessionsMu.Unlock()

	sess.mu.Lock()
	defer sess.mu.Unlock()

	// Check termination conditions
	if sess.closed {
		return exitResult(sess.turnCount, int(time.Since(sess.startedAt).Seconds()), sess.callSID, sess.streamSID), nil
	}
	if sess.turnCount >= sess.maxTurns {
		cleanupSession(sessionID)
		return exitResult(sess.turnCount, int(time.Since(sess.startedAt).Seconds()), sess.callSID, sess.streamSID), nil
	}
	if time.Since(sess.startedAt) >= sess.maxDur {
		cleanupSession(sessionID)
		return exitResult(sess.turnCount, int(time.Since(sess.startedAt).Seconds()), sess.callSID, sess.streamSID), nil
	}

	// Collect speech audio via VAD
	sess.vad.Reset()
	var audioBuffer []byte
	maxBufferSize := 8000 * 60 // 60 seconds of mulaw at 8kHz

	for {
		sess.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, rawMsg, err := sess.conn.ReadMessage()
		if err != nil {
			log.WithFields(log.Fields{
				"session_id": sessionID,
				"error":      err,
			}).Warn("voice session WebSocket read error")
			sess.closed = true
			cleanupSession(sessionID)
			return exitResult(sess.turnCount, int(time.Since(sess.startedAt).Seconds()), sess.callSID, sess.streamSID), nil
		}

		var msg wsMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			continue
		}

		switch msg.Event {
		case "media":
			if msg.Media == nil || msg.Media.Payload == "" {
				continue
			}
			audioData, err := base64.StdEncoding.DecodeString(msg.Media.Payload)
			if err != nil {
				continue
			}

			vadResult := sess.vad.ProcessFrame(audioData)
			if vadResult == twilio.VADSpeech || vadResult == twilio.VADEndOfSpeech {
				audioBuffer = append(audioBuffer, audioData...)
			}

			if vadResult == twilio.VADEndOfSpeech && len(audioBuffer) > 0 {
				// Speech segment complete
				sess.turnCount++
				wavAudio := twilio.WrapMulawInWAV(audioBuffer)
				encoded := base64.StdEncoding.EncodeToString(wavAudio)
				return map[string]interface{}{
					"result":             true,
					"voice_audio_base64": encoded,
					"voice_audio_format": "wav_ulaw_8000",
					"turn_number":        int64(sess.turnCount),
					"current_index":      int64(sess.turnCount - 1),
					"iterations":         int64(sess.turnCount),
					"max_iterations":     int64(sess.maxTurns),
					"call_duration":      int64(time.Since(sess.startedAt).Seconds()),
					"total_turns":        int64(sess.turnCount),
					"call_sid":           sess.callSID,
					"stream_sid":         sess.streamSID,
				}, nil
			}

			// Prevent buffer overflow
			if len(audioBuffer) >= maxBufferSize {
				sess.turnCount++
				wavAudio := twilio.WrapMulawInWAV(audioBuffer)
				encoded := base64.StdEncoding.EncodeToString(wavAudio)
				return map[string]interface{}{
					"result":             true,
					"voice_audio_base64": encoded,
					"voice_audio_format": "wav_ulaw_8000",
					"turn_number":        int64(sess.turnCount),
					"current_index":      int64(sess.turnCount - 1),
					"iterations":         int64(sess.turnCount),
					"max_iterations":     int64(sess.maxTurns),
					"call_duration":      int64(time.Since(sess.startedAt).Seconds()),
					"total_turns":        int64(sess.turnCount),
					"call_sid":           sess.callSID,
					"stream_sid":         sess.streamSID,
				}, nil
			}

		case "stop":
			sess.closed = true
			cleanupSession(sessionID)
			return exitResult(sess.turnCount, int(time.Since(sess.startedAt).Seconds()), sess.callSID, sess.streamSID), nil

		case "dtmf":
			// TODO: handle DTMF input in future phase
			continue

		case "mark":
			// Playback completed — continue listening
			continue
		}
	}
}

// waitForStart reads WebSocket messages until the "start" event arrives.
func waitForStart(sess *activeSession) error {
	sess.conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	for {
		_, rawMsg, err := sess.conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("WebSocket read error waiting for start: %w", err)
		}

		var msg wsMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			continue
		}

		switch msg.Event {
		case "connected":
			continue
		case "start":
			if msg.Start != nil {
				sess.streamSID = msg.Start.StreamSID
				sess.callSID = msg.Start.CallSID
			}
			return nil
		default:
			continue
		}
	}
}

// exitResult returns outputs that signal the loop should stop.
func exitResult(turns, duration int, callSID, streamSID string) map[string]interface{} {
	return map[string]interface{}{
		"result":             false,
		"voice_audio_base64": "",
		"voice_audio_format": "",
		"turn_number":        int64(turns),
		"current_index":      int64(turns),
		"iterations":         int64(turns),
		"max_iterations":     int64(0),
		"call_duration":      int64(duration),
		"total_turns":        int64(turns),
		"call_sid":           callSID,
		"stream_sid":         streamSID,
	}
}

// CleanupSessionByID closes the WebSocket and removes the session from the registry.
// Exported for use by the end_call action.
func CleanupSessionByID(sessionID string) {
	cleanupSession(sessionID)
}

// cleanupSession closes the WebSocket and removes the session from the registry.
func cleanupSession(sessionID string) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	if sess, ok := sessions[sessionID]; ok {
		_ = sess.conn.Close()
		delete(sessions, sessionID)
	}
}

// GetSession returns an active session by ID (used by send_audio action).
func GetSession(sessionID string) *activeSession {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	return sessions[sessionID]
}

// SendAudioToStream sends mulaw audio data back to the caller via the WebSocket.
func (s *activeSession) SendAudioToStream(audioData []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || s.conn == nil {
		return fmt.Errorf("session is closed")
	}

	// Strip WAV headers if present
	audioData = twilio.StripWAVHeader(audioData)

	// Chunk into 20ms frames (160 bytes at 8kHz mulaw)
	chunks := twilio.ChunkAudio(audioData, 160)

	for _, chunk := range chunks {
		payload := base64.StdEncoding.EncodeToString(chunk)
		msg := map[string]interface{}{
			"event":     "media",
			"streamSid": s.streamSID,
			"media": map[string]string{
				"payload": payload,
			},
		}
		data, _ := json.Marshal(msg)
		if err := s.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return fmt.Errorf("failed to send audio: %w", err)
		}
	}

	// Send mark to detect playback completion
	markMsg := map[string]interface{}{
		"event":     "mark",
		"streamSid": s.streamSID,
		"mark": map[string]string{
			"name": fmt.Sprintf("turn_%d", s.turnCount),
		},
	}
	markData, _ := json.Marshal(markMsg)
	return s.conn.WriteMessage(websocket.TextMessage, markData)
}

// ClearPlayback sends a clear message to interrupt audio playback (barge-in).
func (s *activeSession) ClearPlayback() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || s.conn == nil {
		return nil
	}

	msg := map[string]interface{}{
		"event":     "clear",
		"streamSid": s.streamSID,
	}
	data, _ := json.Marshal(msg)
	return s.conn.WriteMessage(websocket.TextMessage, data)
}

// buildWSURL converts an HTTP API URL to a WebSocket URL for the voice session.
func buildWSURL(apiURL, sessionID string) string {
	wsURL := strings.Replace(apiURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	return fmt.Sprintf("%s/api/v1/internal/voice-session/%s", wsURL, sessionID)
}

func optStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func optInt(name string, inputs []*core.Connection, defaultVal int) int {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return defaultVal
	}
	v, err := strconv.Atoi(*c.String())
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}

func optFloat(name string, inputs []*core.Connection, defaultVal float64) float64 {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return defaultVal
	}
	v, err := strconv.ParseFloat(*c.String(), 64)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}

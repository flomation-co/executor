package elevenlabs_common

import (
	"time"

	core "flomation.app/automate/executor"
)

const (
	// APIURL is the base URL for the ElevenLabs API.
	APIURL = "https://api.elevenlabs.io/v1"

	// RequestTimeout is the HTTP client timeout for ElevenLabs API calls.
	// TTS can take a while for longer text, so we use a generous timeout.
	RequestTimeout = 120 * time.Second

	// MaxResponseBody caps the response body to prevent memory exhaustion.
	MaxResponseBody = 50 << 20 // 50 MB (audio files can be large)
)

// GetAPIKey extracts and validates the API key from action inputs.
func GetAPIKey(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("api_key", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return "", nil
	}
	return *conn.String(), nil
}

// OptionalString extracts a string input, returning empty string if absent.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}

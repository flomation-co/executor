package webflow_common

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// APIURL is the base URL for the Webflow v2 API.
	APIURL = "https://api.webflow.com/v2"

	// maxResponseBody caps the response body to prevent memory exhaustion.
	maxResponseBody = 1 << 20 // 1 MB

	// requestTimeout is the HTTP client timeout for Webflow API calls.
	requestTimeout = 30 * time.Second
)

// AuthInputs returns the shared API token input used by all Webflow actions.
var AuthInputs = []core.Connection{
	{
		Name:        "api_token",
		Type:        core.ConnectionTypeString,
		Label:       "Webflow API Token",
		Placeholder: "wfl_...",
		Required:    true,
	},
}

// ExecuteRequest makes an HTTP request to the Webflow v2 API with Bearer
// authentication and returns the status code and response body.
func ExecuteRequest(token, method, path string, body []byte) (int, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	httpReq, err := http.NewRequest(method, APIURL+path, reqBody)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, nil, fmt.Errorf("Webflow API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to read response: %w", err)
	}

	return resp.StatusCode, respBody, nil
}

// GetAPIToken extracts and validates the API token from action inputs.
func GetAPIToken(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("api_token", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return "", fmt.Errorf("api_token is required")
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

// RequiredString extracts a required string input, returning an error if absent.
func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// OptionalInt extracts an integer input, returning 0 if absent.
func OptionalInt(name string, inputs []*core.Connection) int {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Number() == nil {
		return 0
	}
	return int(*conn.Number())
}

// OptionalBool extracts a boolean input, returning the given default if absent.
func OptionalBool(name string, inputs []*core.Connection, defaultVal bool) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return defaultVal
	}
	return *conn.Boolean()
}

// ErrorResult returns a standardised error result map for failed actions.
func ErrorResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}, nil
}

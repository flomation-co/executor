package linkedin_common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
)

const (
	BaseURL        = "https://api.linkedin.com/v2"
	RestBaseURL    = "https://api.linkedin.com/rest"
	requestTimeout = 30 * time.Second
	maxResponse    = 1 << 20 // 1 MB
)

// AuthInputs are the shared inputs for all LinkedIn actions.
var AuthInputs = []core.Connection{
	{
		Name:        "access_token",
		Type:        core.ConnectionTypeString,
		Label:       "LinkedIn Access Token",
		Placeholder: "${credentials.linkedin}",
		Required:    true,
	},
}

// APIResponse wraps an HTTP response from the LinkedIn API.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ExecuteAPI performs a REST call to the LinkedIn API.
func ExecuteAPI(accessToken, method, url string, body interface{}) (*APIResponse, error) {
	var bodyReader io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	req.Header.Set("LinkedIn-Version", "202405")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LinkedIn API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return &APIResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Headers:    resp.Header,
	}, nil
}

// CheckResponse verifies the HTTP status is 2xx.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var errResp struct {
		Message string `json:"message"`
		Status  int    `json:"status"`
	}
	if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Message != "" {
		return fmt.Errorf("LinkedIn API error (%d): %s", resp.StatusCode, errResp.Message)
	}

	return fmt.Errorf("LinkedIn API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// GetAccessToken extracts the access token from inputs.
func GetAccessToken(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("access_token", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return "", fmt.Errorf("access_token is required")
	}
	return *conn.String(), nil
}

// OptionalString extracts an optional string input.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}

// ErrorResult returns a standard error output map.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// SuccessResult returns a standard success output map.
func SuccessResult(toolResult string, extra map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"tool_result": toolResult,
		"success":     true,
		"error":       "",
	}
	for k, v := range extra {
		result[k] = v
	}
	return result
}

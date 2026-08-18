package facebook_common

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

// GraphAPIBase is the Graph API root. A var, not a const, so tests can point it
// at an httptest server — this package had no execution tests at all because it
// could not be redirected.
var GraphAPIBase = "https://graph.facebook.com/v25.0"

const (
	requestTimeout = 30 * time.Second
	maxResponse    = 1 << 20 // 1 MB
)

// AuthInputs are the shared inputs for all Facebook actions.
var AuthInputs = []core.Connection{
	{
		Name:        "access_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Facebook Access Token",
		Placeholder: "${credentials.facebook}",
		Required:    true,
	},
	{
		Name:        "app_secret",
		Type:        core.ConnectionTypeSecret,
		Label:       "App Secret",
		Placeholder: "${secrets.facebook_app_secret}",
	},
}

// APIResponse wraps an HTTP response from the Graph API.
type APIResponse struct {
	StatusCode int
	Body       []byte
}

// ExecuteAPI performs a GET/POST/DELETE call to the Facebook Graph API.
func ExecuteAPI(accessToken, appSecret, method, endpoint string, params url.Values) (*APIResponse, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("access_token", accessToken)

	// Add appsecret_proof — required when "Require App Secret" is enabled
	if appSecret != "" {
		mac := hmac.New(sha256.New, []byte(appSecret))
		mac.Write([]byte(accessToken))
		params.Set("appsecret_proof", hex.EncodeToString(mac.Sum(nil)))
	}

	var apiURL string
	var bodyReader io.Reader

	if method == http.MethodGet || method == http.MethodDelete {
		apiURL = endpoint + "?" + params.Encode()
	} else {
		apiURL = endpoint
		bodyReader = strings.NewReader(params.Encode())
	}

	req, err := http.NewRequest(method, apiURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Graph API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return &APIResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
	}, nil
}

// CheckResponse verifies the HTTP status is 2xx.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Errorf("Graph API error (%d): %s", errResp.Error.Code, errResp.Error.Message)
	}

	return fmt.Errorf("Graph API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// GetAccessToken extracts the access token from inputs.
func GetAccessToken(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("access_token", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return "", fmt.Errorf("access_token is required")
	}
	return *conn.String(), nil
}

// GetAppSecret extracts the app secret from inputs.
func GetAppSecret(inputs []*core.Connection) string {
	return OptionalString("app_secret", inputs)
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

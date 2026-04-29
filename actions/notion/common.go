package notion_common

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
	// BaseURL is the Notion API endpoint.
	BaseURL = "https://api.notion.com/v1"

	// NotionVersion is the API version header value.
	NotionVersion = "2022-06-28"

	// maxResponseBody caps the response body to prevent memory exhaustion.
	maxResponseBody = 1 << 20 // 1 MB

	// requestTimeout is the HTTP client timeout for Notion API calls.
	requestTimeout = 30 * time.Second
)

// AuthInputs are the shared connection inputs used by every Notion action.
var AuthInputs = []core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeString,
		Label:       "Notion Integration Token",
		Placeholder: "ntn_...",
		Required:    true,
	},
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ExecuteAPI performs a REST call to the Notion API.
// method: GET, POST, PATCH, DELETE
// path: relative to /v1 (e.g. "/pages/abc-123")
// body: optional payload — marshalled to JSON for POST/PATCH, ignored for GET/DELETE
func ExecuteAPI(apiKey, method, path string, body interface{}) (*APIResponse, error) {
	fullURL := BaseURL + path

	var bodyReader io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPatch) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Notion-Version", NotionVersion)
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Notion API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &APIResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Headers:    resp.Header,
	}, nil
}

// CheckResponse verifies the status code is in the 2xx range.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var errResp struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Message != "" {
		return fmt.Errorf("Notion API error (%d/%s): %s", resp.StatusCode, errResp.Code, errResp.Message)
	}

	return fmt.Errorf("Notion API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// GetAPIKey extracts and validates the API key from action inputs.
func GetAPIKey(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("api_key", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return "", fmt.Errorf("api_key is required")
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

// ErrorResult returns a standard error output map for action failures.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ExtractTitle extracts a plain-text title from a Notion page properties object.
func ExtractTitle(properties map[string]interface{}) string {
	for _, v := range properties {
		prop, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if prop["type"] != "title" {
			continue
		}
		titleArr, ok := prop["title"].([]interface{})
		if !ok || len(titleArr) == 0 {
			continue
		}
		if rt, ok := titleArr[0].(map[string]interface{}); ok {
			if text, ok := rt["plain_text"].(string); ok {
				return text
			}
		}
	}
	return ""
}

// ExtractRichText extracts plain text from a Notion rich_text array.
func ExtractRichText(arr []interface{}) string {
	var result string
	for _, item := range arr {
		if rt, ok := item.(map[string]interface{}); ok {
			if text, ok := rt["plain_text"].(string); ok {
				result += text
			}
		}
	}
	return result
}

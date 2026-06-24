package databricks_common

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
)

const (
	// maxResponseBody caps the response body. INLINE result sets can be up to
	// 25 MiB, so allow some headroom above that.
	maxResponseBody = 32 << 20 // 32 MB

	// requestTimeout is the HTTP client timeout for a single Databricks API call.
	// The statement endpoint can block server-side for up to wait_timeout (30s),
	// so this is comfortably above that.
	requestTimeout = 90 * time.Second
)

// AuthInputs are the shared connection inputs used by every Databricks action.
var AuthInputs = []core.Connection{
	{
		Name:        "host",
		Type:        core.ConnectionTypeString,
		Label:       "Workspace URL",
		Placeholder: "https://dbc-xxxxxxxx.cloud.databricks.com",
		Required:    true,
	},
	{
		Name:        "token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Access Token (PAT)",
		Placeholder: "dapi...",
		Required:    true,
	},
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// NormaliseHost trims trailing slashes and ensures the workspace URL has an
// https scheme, so callers may paste the bare host or a full URL.
func NormaliseHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimRight(host, "/")
	if host == "" {
		return ""
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	return host
}

// ExecuteAPI performs a REST call against the Databricks workspace API.
// method: GET, POST, DELETE
// path: absolute API path beginning with "/" (e.g. "/api/2.0/sql/statements/")
// body: optional payload — marshalled to JSON for POST, ignored otherwise.
func ExecuteAPI(host, token, method, path string, body interface{}) (*APIResponse, error) {
	fullURL := NormaliseHost(host) + path

	var bodyReader io.Reader
	if body != nil && method == http.MethodPost {
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

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Databricks API request failed: %w", err)
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

// ExecuteRaw performs a REST call with a raw (non-JSON) request body, used by
// the Files API where the body is the file's bytes. A nil body sends no body.
func ExecuteRaw(host, token, method, path, contentType string, body []byte) (*APIResponse, error) {
	fullURL := NormaliseHost(host) + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Databricks API request failed: %w", err)
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

// EncodePath percent-encodes each segment of a Files API path while preserving
// the slashes, so a path like "/Volumes/main/default/vol/my file.csv" is safe
// to embed in a request URL.
func EncodePath(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

// CheckResponse verifies the status code is in the 2xx range, surfacing the
// Databricks error message where available.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var errResp struct {
		Message   string `json:"message"`
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Message != "" {
		return fmt.Errorf("Databricks API error (%d/%s): %s", resp.StatusCode, errResp.ErrorCode, errResp.Message)
	}

	return fmt.Errorf("Databricks API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// GetAuth extracts and validates the workspace host and access token.
func GetAuth(inputs []*core.Connection) (host, token string, err error) {
	host, err = RequiredString("host", inputs)
	if err != nil {
		return "", "", err
	}
	token, err = RequiredString("token", inputs)
	if err != nil {
		return "", "", err
	}
	return host, token, nil
}

// OptionalString extracts a string input, returning empty string if absent.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}

// RequiredString extracts a required string input, returning an error if absent.
func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// OptionalInt extracts an integer input, reporting whether it was present.
func OptionalInt(name string, inputs []*core.Connection) (int64, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Number() == nil {
		return 0, false
	}
	return *conn.Number(), true
}

// RequiredInt extracts a required integer input, returning an error if absent.
func RequiredInt(name string, inputs []*core.Connection) (int64, error) {
	v, ok := OptionalInt(name, inputs)
	if !ok {
		return 0, fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// ErrorResult returns a standard soft-error output map for action failures,
// matching the success/error output ports used across the Jobs actions.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

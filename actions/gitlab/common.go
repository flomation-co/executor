package gitlab_common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// DefaultBaseURL is the SaaS GitLab instance.
	DefaultBaseURL = "https://gitlab.com"

	// APIPath is the v4 REST API prefix.
	APIPath = "/api/v4"

	// maxResponseBody caps the response body to prevent memory exhaustion.
	maxResponseBody = 1 << 20 // 1 MB

	// requestTimeout is the HTTP client timeout for GitLab API calls.
	requestTimeout = 30 * time.Second
)

// AuthInputs are the shared connection inputs used by every GitLab action.
var AuthInputs = []core.Connection{
	{
		Name:        "access_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "GitLab Access Token",
		Placeholder: "glpat-...",
		Required:    true,
	},
	{
		Name:        "base_url",
		Type:        core.ConnectionTypeString,
		Label:       "GitLab Base URL",
		Placeholder: "https://gitlab.com",
	},
	{
		Name:        "project_id",
		Type:        core.ConnectionTypeString,
		Label:       "Project ID",
		Placeholder: "Numeric ID or namespace/project",
		Required:    true,
	},
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ExecuteAPI performs a REST call to the GitLab API.
// method: GET, POST, PUT, DELETE
// path: the full URL path after /api/v4 (e.g. "/projects/8/merge_requests")
// body: optional payload — marshalled to JSON for POST/PUT, ignored for GET/DELETE
func ExecuteAPI(accessToken, baseURL, method, path string, body interface{}) (*APIResponse, error) {
	fullURL := BuildURL(baseURL, path)

	var bodyReader io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPut) {
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

	req.Header.Set("PRIVATE-TOKEN", accessToken)
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitLab API request failed: %w", err)
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

// CheckResponse verifies the status code is in the 2xx range and returns
// a user-friendly error if not.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Try to extract GitLab error message
	var errResp struct {
		Message  interface{} `json:"message"`
		ErrorMsg string      `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &errResp); err == nil {
		if errResp.ErrorMsg != "" {
			return fmt.Errorf("GitLab API error (%d): %s", resp.StatusCode, errResp.ErrorMsg)
		}
		if errResp.Message != nil {
			b, _ := json.Marshal(errResp.Message)
			return fmt.Errorf("GitLab API error (%d): %s", resp.StatusCode, string(b))
		}
	}

	return fmt.Errorf("GitLab API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// BuildURL constructs the full API URL from the base URL and path.
func BuildURL(baseURL, path string) string {
	base := strings.TrimRight(baseURL, "/")
	return base + APIPath + path
}

// BuildProjectURL constructs a URL scoped to a specific project.
// path is appended after /projects/:id (e.g. "/merge_requests").
func BuildProjectURL(baseURL, projectID, path string) string {
	return BuildURL(baseURL, fmt.Sprintf("/projects/%s%s", EncodeProjectID(projectID), path))
}

// EncodeProjectID URL-encodes the project ID for use in API paths.
// Numeric IDs pass through unchanged; namespace/project is percent-encoded.
func EncodeProjectID(id string) string {
	if _, err := strconv.Atoi(id); err == nil {
		return id
	}
	return url.PathEscape(id)
}

// GetAccessToken extracts and validates the access token from action inputs.
func GetAccessToken(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("access_token", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return "", fmt.Errorf("access_token is required")
	}
	return *conn.String(), nil
}

// GetBaseURL extracts the base URL, returning DefaultBaseURL if not provided.
func GetBaseURL(inputs []*core.Connection) string {
	conn := core.FindConnection("base_url", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return DefaultBaseURL
	}
	return strings.TrimRight(*conn.String(), "/")
}

// GetProjectID extracts and validates the project ID from action inputs.
func GetProjectID(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("project_id", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return "", fmt.Errorf("project_id is required")
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

// OptionalInt extracts an optional integer input.
func OptionalInt(name string, inputs []*core.Connection) *int64 {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Number() == nil {
		return nil
	}
	n := *conn.Number()
	return &n
}

// OptionalBool extracts an optional boolean input.
func OptionalBool(name string, inputs []*core.Connection) *bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return nil
	}
	b := *conn.Boolean()
	return &b
}

// ProjectAPI is a convenience wrapper that calls ExecuteAPI scoped to a project.
// path is relative to /projects/:id (e.g. "/merge_requests/1").
func ProjectAPI(token, baseURL, projectID, method, path string, body interface{}) (*APIResponse, error) {
	fullPath := fmt.Sprintf("/projects/%s%s", EncodeProjectID(projectID), path)
	return ExecuteAPI(token, baseURL, method, fullPath, body)
}

// ErrorResult returns a standard error output map for action failures.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

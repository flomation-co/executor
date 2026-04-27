package github_common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// DefaultBaseURL is the GitHub SaaS API endpoint.
	DefaultBaseURL = "https://api.github.com"

	// maxResponseBody caps the response body to prevent memory exhaustion.
	maxResponseBody = 1 << 20 // 1 MB

	// requestTimeout is the HTTP client timeout for GitHub API calls.
	requestTimeout = 30 * time.Second

	// apiVersion is the GitHub REST API version header value.
	apiVersion = "2022-11-28"
)

// AuthInputs are the shared connection inputs used by every GitHub action.
var AuthInputs = []core.Connection{
	{
		Name:        "access_token",
		Type:        core.ConnectionTypeString,
		Label:       "GitHub Access Token",
		Placeholder: "ghp_... or github_pat_...",
		Required:    true,
	},
	{
		Name:        "base_url",
		Type:        core.ConnectionTypeString,
		Label:       "GitHub API Base URL",
		Placeholder: "https://api.github.com",
	},
	{
		Name:        "owner",
		Type:        core.ConnectionTypeString,
		Label:       "Repository Owner",
		Placeholder: "Organisation or username",
		Required:    true,
	},
	{
		Name:        "repo",
		Type:        core.ConnectionTypeString,
		Label:       "Repository Name",
		Required:    true,
	},
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ExecuteAPI performs a REST call to the GitHub API.
// path is the full API path (e.g. "/repos/owner/repo/pulls").
func ExecuteAPI(accessToken, baseURL, method, path string, body interface{}) (*APIResponse, error) {
	fullURL := BuildURL(baseURL, path)

	var bodyReader io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) {
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

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request failed: %w", err)
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
	}
	if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Message != "" {
		return fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, errResp.Message)
	}

	return fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// BuildURL constructs the full API URL from the base URL and path.
// For GitHub Enterprise, if the base URL is not the default, /api/v3 is prepended.
func BuildURL(baseURL, path string) string {
	base := strings.TrimRight(baseURL, "/")
	if base != DefaultBaseURL && !strings.HasSuffix(base, "/api/v3") {
		base += "/api/v3"
	}
	return base + path
}

// RepoAPI is a convenience wrapper that calls ExecuteAPI scoped to a repository.
// path is relative to /repos/{owner}/{repo} (e.g. "/pulls/1").
func RepoAPI(token, baseURL, owner, repo, method, path string, body interface{}) (*APIResponse, error) {
	fullPath := fmt.Sprintf("/repos/%s/%s%s", owner, repo, path)
	return ExecuteAPI(token, baseURL, method, fullPath, body)
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

// GetOwner extracts and validates the repository owner from action inputs.
func GetOwner(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("owner", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return "", fmt.Errorf("owner is required")
	}
	return *conn.String(), nil
}

// GetRepo extracts and validates the repository name from action inputs.
func GetRepo(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("repo", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return "", fmt.Errorf("repo is required")
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

// OptionalBool extracts an optional boolean input.
func OptionalBool(name string, inputs []*core.Connection) *bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return nil
	}
	b := *conn.Boolean()
	return &b
}

// ErrorResult returns a standard error output map for action failures.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// Package microsoft_common provides shared authentication and HTTP helpers
// for all Microsoft 365 actions (Outlook, OneDrive, Excel, Teams, etc.).
//
// Token flow: FetchTokens contacts the Launch service (via API proxy) to
// retrieve OAuth tokens for the specified purpose. The Launch service
// handles token refresh transparently.
package microsoft_common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	// GraphAPI is the base URL for Microsoft Graph API v1.0.
	GraphAPI = "https://graph.microsoft.com/v1.0"

	defaultTimeout = 15 * time.Second
	longTimeout    = 60 * time.Second
	maxResponseLog = 512
	maxRetries     = 3
)

// TokenInfo holds an OAuth token retrieved from the Launch service.
type TokenInfo struct {
	Email       string `json:"email"`
	Label       string `json:"label"`
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

// FetchTokens retrieves Microsoft OAuth tokens for the given purpose.
// If credential is non-empty, it is used as a raw access token (bypassing
// the Launch service). Otherwise, tries agent-user tokens first, then
// falls back to agent-level tokens.
func FetchTokens(flow *core.Flow, credential, purpose string) ([]TokenInfo, error) {
	// Direct credential override
	if credential != "" && !strings.HasPrefix(credential, "${") {
		return []TokenInfo{{AccessToken: credential, Email: "credential-override"}}, nil
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	client := ctx.InternalClient()
	var all []TokenInfo

	// Agent-user scoped tokens (per-user credentials)
	if ctx.AgentUserID != "" {
		endpoint := fmt.Sprintf("%s/api/v1/internal/agent-user/%s/microsoft-tokens?purpose=%s",
			ctx.APIURL, ctx.AgentUserID, purpose)
		if tokens := fetchTokensFrom(flow, client, endpoint); len(tokens) > 0 {
			all = append(all, tokens...)
		}
	}

	// Fall back to agent-level tokens
	if len(all) == 0 && ctx.AgentID != "" {
		endpoint := fmt.Sprintf("%s/api/v1/internal/trigger/%s/microsoft-tokens?purpose=%s",
			ctx.APIURL, ctx.AgentID, purpose)
		if tokens := fetchTokensFrom(flow, client, endpoint); len(tokens) > 0 {
			all = append(all, tokens...)
		}
	}

	if len(all) == 0 {
		return nil, fmt.Errorf("no Microsoft tokens available — connect a Microsoft account with %s permissions", purpose)
	}
	return all, nil
}

func fetchTokensFrom(flow *core.Flow, client *http.Client, endpoint string) []TokenInfo {
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var tokens []TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil
	}
	return tokens
}

// FilterTokens returns only active tokens matching the account filter.
// An empty filter returns all non-errored tokens.
func FilterTokens(tokens []TokenInfo, accountFilter string) []TokenInfo {
	var active []TokenInfo
	for _, t := range tokens {
		if t.Error != "" {
			continue
		}
		if accountFilter != "" {
			if !strings.EqualFold(t.Email, accountFilter) &&
				!strings.EqualFold(t.Label, accountFilter) &&
				!strings.Contains(strings.ToLower(t.Email), strings.ToLower(accountFilter)) {
				continue
			}
		}
		active = append(active, t)
	}
	return active
}

// DoRequest makes an authenticated HTTP request to the Microsoft Graph API.
// Handles 429 rate-limit responses with automatic retry.
func DoRequest(flow *core.Flow, method, url, accessToken string, body []byte) (int, []byte, error) {
	return doRequestWithRetry(flow, method, url, accessToken, body, defaultTimeout)
}

// DoRequestLong makes an authenticated HTTP request with a longer timeout,
// suitable for file uploads and downloads.
func DoRequestLong(flow *core.Flow, method, url, accessToken string, body []byte) (int, []byte, error) {
	return doRequestWithRetry(flow, method, url, accessToken, body, longTimeout)
}

func doRequestWithRetry(flow *core.Flow, method, url, accessToken string, body []byte, timeout time.Duration) (int, []byte, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		status, respBody, err := doRequest(flow, method, url, accessToken, body, timeout)
		if err != nil {
			return 0, nil, err
		}

		if status != http.StatusTooManyRequests || attempt == maxRetries {
			return status, respBody, nil
		}

		// Parse Retry-After header value from response (default 5s)
		wait := 5 * time.Second
		if attempt < maxRetries {
			// The response body is already read; parse retry-after from a simple heuristic
			// Microsoft typically returns seconds in Retry-After
			wait = time.Duration(5*(attempt+1)) * time.Second
		}

		log.WithFields(log.Fields{
			"attempt": attempt + 1,
			"wait":    wait,
			"url":     url,
		}).Warn("[microsoft] rate limited, retrying")

		time.Sleep(wait)
	}

	return http.StatusTooManyRequests, nil, fmt.Errorf("rate limited after %d retries", maxRetries)
}

func doRequest(flow *core.Flow, method, url, accessToken string, body []byte, timeout time.Duration) (int, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), method, url, reqBody)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5 MB limit
	return resp.StatusCode, respBody, nil
}

// HandleAuthError disconnects the Microsoft account if the response indicates
// expired or revoked credentials. Fire-and-forget.
func HandleAuthError(flow *core.Flow, email string, statusCode int) {
	if statusCode != http.StatusUnauthorized && statusCode != http.StatusForbidden {
		return
	}
	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" || ctx.AgentUserID == "" {
		return
	}
	endpoint := fmt.Sprintf("%s/api/v1/internal/agent-user/%s/microsoft-account/%s",
		ctx.APIURL, ctx.AgentUserID, email)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodDelete, endpoint, nil)
	if err != nil {
		return
	}
	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		log.WithError(err).Warn("[microsoft] failed to disconnect expired account")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	log.WithFields(log.Fields{
		"email":  email,
		"status": resp.StatusCode,
	}).Info("[microsoft] disconnected expired account")
}

// ErrorResult returns a standard error output map.
func ErrorResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"success":     false,
		"error":       msg,
	}, nil
}

// TruncateBody returns at most maxResponseLog bytes of the response for error messages.
func TruncateBody(body []byte) string {
	if len(body) > maxResponseLog {
		return string(body[:maxResponseLog])
	}
	return string(body)
}

// OptStr extracts an optional string input value.
func OptStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	s := *c.String()
	if strings.HasPrefix(s, "${") {
		return ""
	}
	return s
}

// OptInt extracts an optional integer input value with a default.
func OptInt(name string, inputs []*core.Connection, defaultVal int) int {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return defaultVal
	}

	// Try number type first
	if c.Number() != nil {
		v := int(*c.Number())
		if v <= 0 {
			return defaultVal
		}
		return v
	}

	// Fall back to string parsing (editor stores some values as strings)
	if c.String() != nil {
		s := *c.String()
		if strings.HasPrefix(s, "${") {
			return defaultVal
		}
		v, err := strconv.Atoi(s)
		if err != nil || v <= 0 {
			return defaultVal
		}
		return v
	}

	return defaultVal
}

// OptBool extracts an optional boolean input value.
func OptBool(name string, inputs []*core.Connection) bool {
	return OptStr(name, inputs) == "true"
}

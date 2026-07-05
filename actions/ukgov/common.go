// Package ukgov_common provides shared HTTP plumbing and input helpers for the
// UK Government agency actions (Companies House, DVLA, Police, Food Standards,
// Environment Agency and postcode lookups).
//
// These agencies expose read-only JSON APIs spanning three authentication
// styles: none (Police, FSA, Environment Agency, postcodes.io), an API key in a
// request header (DVLA), and HTTP Basic auth with the key as the username and a
// blank password (Companies House). Centralising the request plumbing keeps
// each action down to building a URL, supplying any auth headers, and shaping
// its outputs.
//
// This package intentionally has no Execute function, so the manifest generator
// treats it as a category holder (like git_common) rather than an action.
package ukgov_common

import (
	"bytes"
	"context"
	"encoding/base64"
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
	// RequestTimeout bounds every outbound HTTP call.
	RequestTimeout = 30 * time.Second

	// MaxResponseBody caps the response body read to prevent memory exhaustion
	// from an unexpectedly large or hostile response.
	MaxResponseBody = 4 << 20 // 4 MB
)

// Fetch performs a bounded HTTP GET-style request (no body) and returns the
// status code and body. It reads at most MaxResponseBody bytes. Callers own
// status-code interpretation (e.g. mapping 404 to a friendly "not found" tool
// result) and JSON decoding, keeping this helper agnostic to each agency's
// payload shape. A nil context is tolerated so actions can call it with a nil
// Flow during testing.
func Fetch(ctx context.Context, method, requestURL string, headers map[string]string) (int, []byte, error) {
	return FetchWithBody(ctx, method, requestURL, headers, nil)
}

// FetchWithBody is Fetch with an optional request body (e.g. a JSON POST, as
// the DVLA Vehicle Enquiry Service requires). When body is non-nil the
// Content-Type is set to application/json. The body is bounded on read exactly
// as Fetch.
func FetchWithBody(ctx context.Context, method, requestURL string, headers map[string]string, body []byte) (int, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, requestURL, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBody))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return resp.StatusCode, respBody, nil
}

// TokenResponse is the OAuth2 token endpoint response envelope.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// ClientCredentialsToken performs an OAuth2 client-credentials grant
// (application/x-www-form-urlencoded) against tokenURL and returns the parsed
// token. Used by agencies fronted by an OAuth2 provider — e.g. DVSA MOT history
// via Microsoft Entra ID. The executor runs as a fresh process per execution,
// so callers fetch a token once per execution rather than caching across runs.
func ClientCredentialsToken(ctx context.Context, tokenURL, clientID, clientSecret, scope string) (*TokenResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", scope)

	reqCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tok TokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint returned an empty access_token")
	}
	return &tok, nil
}

// BasicAuthHeader builds an HTTP Basic Authorization header value. Companies
// House authenticates with the API key as the username and a blank password.
func BasicAuthHeader(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

// OptionalString returns a string input, or "" if it is absent or unset.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}

// RequiredString returns a string input, or an error if it is empty.
func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// OptionalInt returns an integer input, falling back to def if it is absent or
// cannot be parsed.
func OptionalInt(name string, inputs []*core.Connection, def int) int {
	conn := core.FindConnection(name, inputs)
	if conn == nil {
		return def
	}
	n := conn.Number()
	if n == nil {
		return def
	}
	return int(*n)
}

// ErrResult is the standard failure return for a UK Government action. It
// returns a nil Go error so the message surfaces to the AI via tool_result
// while the node is still marked unsuccessful — the AI-native action convention.
func ErrResult(format string, args ...interface{}) (map[string]interface{}, error) {
	msg := fmt.Sprintf(format, args...)
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}, nil
}

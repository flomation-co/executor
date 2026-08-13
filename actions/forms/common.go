package forms_common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// RequestTimeout bounds every outbound HTTP call to a form provider.
	RequestTimeout = 30 * time.Second

	// MaxResponseBody caps the response body read to prevent memory exhaustion
	// from an unexpectedly large or hostile response.
	MaxResponseBody = 8 << 20 // 8 MB
)

// DoJSON performs a bounded HTTP request against a form provider REST API using
// a bearer token, and returns the status code and raw body. When body is
// non-nil the request is sent as application/json. It reads at most
// MaxResponseBody bytes. Callers own status-code interpretation and JSON
// decoding, keeping this helper agnostic to each provider's payload shape. A
// nil context is tolerated so actions can call it with a nil Flow during
// testing.
func DoJSON(ctx context.Context, method, requestURL, bearer string, body []byte) (int, []byte, error) {
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
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
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

// Fetch is a convenience wrapper for a bearer-authenticated request with no
// body (GET/DELETE-style calls).
func Fetch(ctx context.Context, method, requestURL, bearer string) (int, []byte, error) {
	return DoJSON(ctx, method, requestURL, bearer, nil)
}

// --- input helpers ---

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

// --- result shapers (mirror the stripe / ukgov contract; tool_result FIRST) ---

// summaryWithData embeds the JSON payload in tool_result because the engine's
// tool-result fallback chain (tool_result -> result -> response -> output ->
// JSON of all outputs) never falls through a non-empty tool_result: an AI
// caller would otherwise get the human summary but not the data. The engine
// truncates large payloads downstream, so embedding the full data here is safe.
func summaryWithData(summary string, data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil || len(b) == 0 || string(b) == "null" {
		return summary
	}
	if summary == "" {
		return string(b)
	}
	return summary + "\n" + string(b)
}

// ObjectResult wraps a single object result. The object is provided as a plain
// map so downstream nodes can reach ${input.result.<field>}.
func ObjectResult(obj map[string]interface{}, summary string) map[string]interface{} {
	id, _ := obj["id"].(string)
	return map[string]interface{}{
		"tool_result": summaryWithData(summary, obj),
		"id":          id,
		"result":      obj,
		"success":     true,
		"error":       "",
	}
}

// ListResult wraps a list of objects.
func ListResult(items []map[string]interface{}, summary string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": summaryWithData(summary, items),
		"results":     items,
		"count":       len(items),
		"success":     true,
		"error":       "",
	}
}

// ErrorResult is a graceful failure — success=false with a nil Go error — so the
// message surfaces to the AI via tool_result while the node is still marked
// unsuccessful (the AI-native action convention).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// DecodeMap JSON-decodes a response body into a plain map, returning an empty
// map on error.
func DecodeMap(body []byte) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

// Package jotform holds the shared auth inputs, request wrapper and helpers for
// the JotForm actions. It has no Execute function, so the manifest generator
// excludes it from the action registry (it is a sub-category holder).
package jotform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
)

// regionBaseURLs maps a JotForm data-centre region to its API root. JotForm
// hosts EU and HIPAA accounts on dedicated regional endpoints; the wrong host
// returns authentication failures for a valid key.
var regionBaseURLs = map[string]string{
	"us":    "https://api.jotform.com",
	"eu":    "https://eu-api.jotform.com",
	"hipaa": "https://hipaa-api.jotform.com",
}

// AuthInputs are the shared credential inputs for every JotForm action: an API
// key (sent in the APIKEY header) and a region selector. Supplied by the flow
// author as an environment secret (${secrets.jotform_api_key}).
//
// Note: this var is inlined into each action's Inputs literal rather than
// shared from here at manifest-generation time — the manifest generator only
// resolves literal composite literals, so a shared reference would be skipped.
// Kept here for documentation and for the Go compiler to reuse in Execute paths.
var AuthInputs = []core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "JotForm API Key", Placeholder: "${secrets.jotform_api_key}", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "us", Options: []core.ConnectionOption{
		{Name: "US (default)", Value: "us"},
		{Name: "EU", Value: "eu"},
		{Name: "HIPAA", Value: "hipaa"},
	}},
}

// Context returns the flow's Go context, tolerating a nil flow (as in tests).
func Context(flow *core.Flow) context.Context {
	if flow != nil {
		return flow.GoContext()
	}
	return context.Background()
}

// Get returns the API key from the "api_key" input.
func Get(inputs []*core.Connection) (string, error) {
	return forms_common.RequiredString("api_key", inputs)
}

// Region returns the selected region from the "region" input, defaulting to
// "us" when blank or unrecognised.
func Region(inputs []*core.Connection) string {
	r := strings.ToLower(strings.TrimSpace(forms_common.OptionalString("region", inputs)))
	if _, ok := regionBaseURLs[r]; ok {
		return r
	}
	return "us"
}

// resolveBaseURL returns the API root for a region. When BaseURL is set (tests)
// it always wins; otherwise the region map is consulted, defaulting to US.
func resolveBaseURL(region string) string {
	if BaseURL != "" {
		return BaseURL
	}
	if base, ok := regionBaseURLs[strings.ToLower(strings.TrimSpace(region))]; ok {
		return base
	}
	return regionBaseURLs["us"]
}

// Do performs an authenticated JSON request against the JotForm API. path is
// joined to the region-derived base (e.g. "/form/123"). body is nil for
// GET/DELETE calls and marshalled JSON for writes. Returns the parsed JSON map
// (JotForm wraps payloads as {"responseCode":200,"content":...}), the HTTP
// status code, and any transport error.
func Do(ctx context.Context, method, path, apiKey, region string, body []byte) (map[string]interface{}, int, error) {
	contentType := ""
	if body != nil {
		contentType = "application/json"
	}
	return doRequest(ctx, method, path, apiKey, region, contentType, body)
}

// DoForm performs an authenticated application/x-www-form-urlencoded request.
// JotForm's webhook-create endpoint expects form-encoded parameters rather than
// a JSON body, so writes to it go through here.
func DoForm(ctx context.Context, method, path, apiKey, region string, form url.Values) (map[string]interface{}, int, error) {
	return doRequest(ctx, method, path, apiKey, region, "application/x-www-form-urlencoded", []byte(form.Encode()))
}

// doRequest is the shared low-level request builder. It sets the APIKEY header
// (JotForm's auth scheme) and bounds the response read using the same limits as
// forms_common. A nil context is tolerated for testing.
func doRequest(ctx context.Context, method, path, apiKey, region, contentType string, body []byte) (map[string]interface{}, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, forms_common.RequestTimeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, resolveBaseURL(region)+path, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("APIKEY", apiKey)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	client := &http.Client{Timeout: forms_common.RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, forms_common.MaxResponseBody))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}
	return forms_common.DecodeMap(raw), resp.StatusCode, nil
}

// Content surfaces the "content" payload from a JotForm envelope
// ({"responseCode":200,"content":...}) as a map, or an empty map when the
// content is absent or not an object.
func Content(raw map[string]interface{}) map[string]interface{} {
	if m, ok := raw["content"].(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

// ContentList surfaces the "content" payload as a slice of maps, or an empty
// slice when the content is absent or not an array.
func ContentList(raw map[string]interface{}) []map[string]interface{} {
	items := make([]map[string]interface{}, 0)
	if arr, ok := raw["content"].([]interface{}); ok {
		for _, it := range arr {
			if m, ok := it.(map[string]interface{}); ok {
				items = append(items, m)
			}
		}
	}
	return items
}

// StatusMessage maps a non-success JotForm HTTP status to a friendly,
// AI-readable message, preferring the API's own "message" field when present.
func StatusMessage(status int, raw map[string]interface{}) string {
	if msg, ok := raw["message"].(string); ok && msg != "" {
		return msg
	}
	switch status {
	case 401, 403:
		return "JotForm authentication failed — check the API key and the selected region."
	case 404:
		return "JotForm resource not found."
	case 429:
		return "JotForm rate limit exceeded. Try again shortly."
	default:
		return "JotForm returned an unexpected status."
	}
}

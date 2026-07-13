// Package typeform holds the shared auth input, request wrapper and helpers for
// the Typeform actions. It has no Execute function, so the manifest generator
// excludes it from the action registry (it is a sub-category holder).
package typeform

import (
	"context"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
)

// AuthInputs is the shared credential input (a Typeform Personal Access Token).
// Supplied by the flow author as an environment secret (${secrets.X}).
//
// Note: this var is inlined into each action's Inputs literal rather than shared
// from here at manifest-generation time — the manifest generator only resolves
// literal composite literals, so a shared reference would be skipped. Kept here
// for documentation and for the Go compiler to reuse in Execute paths.
var AuthInputs = []core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Typeform Personal Access Token", Placeholder: "${secrets.typeform_token}", Required: true},
}

// Context returns the flow's Go context, tolerating a nil flow (as in tests).
func Context(flow *core.Flow) context.Context {
	if flow != nil {
		return flow.GoContext()
	}
	return context.Background()
}

// Get returns the Personal Access Token from the "api_key" input.
func Get(inputs []*core.Connection) (string, error) {
	return forms_common.RequiredString("api_key", inputs)
}

// Do performs an authenticated request against the Typeform API. path is joined
// to BaseURL (e.g. "/forms/abc123"). body is nil for GET/DELETE calls. Returns
// the parsed JSON map, the HTTP status code, and any transport error.
func Do(ctx context.Context, method, path, token string, body []byte) (map[string]interface{}, int, error) {
	status, raw, err := forms_common.DoJSON(ctx, method, BaseURL+path, token, body)
	if err != nil {
		return nil, status, err
	}
	return forms_common.DecodeMap(raw), status, nil
}

// StatusMessage maps a non-success Typeform HTTP status to a friendly,
// AI-readable message.
func StatusMessage(status int, raw map[string]interface{}) string {
	if msg, ok := raw["description"].(string); ok && msg != "" {
		return msg
	}
	switch status {
	case 401, 403:
		return "Typeform authentication failed — check the Personal Access Token and its scopes."
	case 404:
		return "Typeform resource not found."
	case 429:
		return "Typeform rate limit exceeded. Try again shortly."
	default:
		return "Typeform returned an unexpected status."
	}
}

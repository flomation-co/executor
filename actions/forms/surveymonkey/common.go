// Package surveymonkey holds the shared auth input, request wrapper and helpers
// for the SurveyMonkey actions. It has no Execute function, so the manifest
// generator excludes it from the action registry (it is a sub-category holder).
package surveymonkey

import (
	"context"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
)

// AuthInputs is the shared credential input (a SurveyMonkey long-lived access
// token). Supplied by the flow author as an environment secret (${secrets.X}).
//
// Note: this var is inlined into each action's Inputs literal rather than shared
// from here at manifest-generation time — the manifest generator only resolves
// literal composite literals, so a shared reference would be skipped. Kept here
// for documentation and for the Go compiler to reuse in Execute paths.
var AuthInputs = []core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "SurveyMonkey Access Token", Placeholder: "${secrets.surveymonkey_token}", Required: true},
}

// Context returns the flow's Go context, tolerating a nil flow (as in tests).
func Context(flow *core.Flow) context.Context {
	if flow != nil {
		return flow.GoContext()
	}
	return context.Background()
}

// Get returns the access token from the "access_token" input.
func Get(inputs []*core.Connection) (string, error) {
	return forms_common.RequiredString("access_token", inputs)
}

// Do performs an authenticated Bearer request against the SurveyMonkey API. path
// is joined to BaseURL (e.g. "/surveys/123"). body is nil for GET/DELETE calls.
// Returns the parsed JSON map, the HTTP status code, and any transport error.
func Do(ctx context.Context, method, path, token string, body []byte) (map[string]interface{}, int, error) {
	status, raw, err := forms_common.DoJSON(ctx, method, BaseURL+path, token, body)
	if err != nil {
		return nil, status, err
	}
	return forms_common.DecodeMap(raw), status, nil
}

// StatusMessage maps a non-success SurveyMonkey HTTP status to a friendly,
// AI-readable message. SurveyMonkey nests errors under an "error" object with a
// human-readable "message" field.
func StatusMessage(status int, raw map[string]interface{}) string {
	if errObj, ok := raw["error"].(map[string]interface{}); ok {
		if msg, ok := errObj["message"].(string); ok && msg != "" {
			return msg
		}
	}
	switch status {
	case 401, 403:
		return "SurveyMonkey authentication failed — check the access token and its scopes."
	case 404:
		return "SurveyMonkey resource not found."
	case 429:
		return "SurveyMonkey rate limit exceeded. Try again shortly."
	default:
		return "SurveyMonkey returned an unexpected status."
	}
}

// Package googleforms holds the shared auth inputs, token resolution and
// request wrapper for the Google Forms actions. Unlike the paste-a-key form
// providers (Typeform, JotForm, SurveyMonkey), Google Forms reuses the existing
// Google OAuth managed credential — the same tokens the Google Docs/Sheets
// actions use — resolved via the google_common package.
//
// This package has no Execute function, so the manifest generator excludes it
// from the action registry (it is a sub-category holder, like git_common).
package googleforms

import (
	"fmt"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	google "flomation.app/automate/executor/actions/google"
)

// AuthInputs is the shared Google credential input pair used by every Google
// Forms action: an optional account filter plus the Google OAuth managed
// credential. Mirrors exactly the credential/account inputs used by the Google
// Docs/Sheets actions.
//
// Note: this var is inlined into each action's Inputs literal rather than shared
// from here at manifest-generation time — the manifest generator only resolves
// literal composite literals, so a shared reference would be skipped. Kept here
// for documentation and reuse in the Execute paths.
var AuthInputs = []core.Connection{
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

// Token resolves the active Google access token for the given inputs. It reads
// the optional raw-token/${credentials.X} "credential" input and the optional
// "account" filter, fetches tokens via google_common (which transparently
// refreshes via the Launch service, or treats a non-${...} credential value as a
// raw access token), and selects the first active account. Surfaces refresh
// failures verbatim so the caller can tell the user which account to re-link.
func Token(flow *core.Flow, inputs []*core.Connection) (google.TokenInfo, error) {
	credential := google.OptStr("credential", inputs)
	account := google.OptStr("account", inputs)

	tokens, err := google.FetchTokens(flow, credential)
	if err != nil {
		return google.TokenInfo{}, err
	}
	active, errored := google.SelectActive(tokens, account)
	if len(active) == 0 {
		if msg := google.FormatTokenErrors(errored); msg != "" {
			return google.TokenInfo{}, fmt.Errorf("%s", msg)
		}
		return google.TokenInfo{}, fmt.Errorf("no active Google account available — connect a Google account with Forms access")
	}
	return active[0], nil
}

// Do issues an authenticated request against the Google Forms API. path is
// joined to BaseURL (e.g. "/forms/abc123"). body is nil for GET calls. Returns
// the parsed JSON map and the HTTP status code. On a 401/403 it fires the
// google_common expired-account cleanup for the supplied token.
func Do(flow *core.Flow, method, path string, token google.TokenInfo, body []byte) (map[string]interface{}, int, error) {
	status, raw, err := google.DoRequest(flow, method, BaseURL+path, token.AccessToken, body)
	if err != nil {
		return nil, status, err
	}
	if status == 401 || status == 403 {
		google.HandleAuthError(flow, token.Email, status)
	}
	return forms_common.DecodeMap(raw), status, nil
}

// StatusMessage maps a non-success Google Forms HTTP status to a friendly,
// AI-readable message, preferring the API's own error.message when present.
func StatusMessage(status int, raw map[string]interface{}) string {
	if e, ok := raw["error"].(map[string]interface{}); ok {
		if msg, ok := e["message"].(string); ok && msg != "" {
			return msg
		}
	}
	switch status {
	case 401, 403:
		return "Google Forms authentication failed — reconnect the Google account with Forms access."
	case 404:
		return "Google Forms resource not found."
	case 429:
		return "Google Forms rate limit exceeded. Try again shortly."
	default:
		return fmt.Sprintf("Google Forms returned status %d.", status)
	}
}

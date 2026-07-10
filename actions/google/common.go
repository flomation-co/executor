// Package google_common provides shared authentication and HTTP helpers
// for all Google Workspace actions (Drive, Sheets, Docs, Slides).
//
// Token flow: FetchTokens contacts the Launch service (via API proxy) to
// retrieve OAuth tokens for the specified purpose. The Launch service
// handles token refresh transparently.
package google_common

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
	log "github.com/sirupsen/logrus"
)

const (
	// Purpose is the OAuth scope identifier for Google Workspace.
	// A single "drive" purpose covers Drive, Sheets, Docs, and Slides.
	Purpose = "drive"

	defaultTimeout = 15 * time.Second
	longTimeout    = 60 * time.Second
	maxResponseLog = 512
)

// TokenInfo holds an OAuth token retrieved from the Launch service.
type TokenInfo struct {
	Email       string `json:"email"`
	Label       string `json:"label"`
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

// FetchTokens retrieves Google OAuth tokens for the Drive/Workspace purpose.
// If credential is non-empty, it is used as a raw access token (bypassing
// the Launch service).
//
// Only the agent-user scoped path applies here — Drive tokens are per-
// user. The previous "fall back to agent-level" branch hit the
// /trigger/:id/google-tokens endpoint with AgentID as the trigger ID,
// which always 404'd/missed and only existed because the agent and
// trigger token URLs share the same path shape. Removed rather than
// "fixed" because there's no second source of Drive tokens to fall
// back to.
func FetchTokens(flow *core.Flow, credential string) ([]TokenInfo, error) {
	// Direct credential override
	if credential != "" && !strings.HasPrefix(credential, "${") {
		return []TokenInfo{{AccessToken: credential, Email: "credential-override"}}, nil
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}
	if ctx.AgentUserID == "" {
		return nil, fmt.Errorf("no agent-user context — Google Drive actions require an agent execution")
	}

	endpoint := fmt.Sprintf("%s/api/v1/internal/agent-user/%s/google-tokens?purpose=%s",
		ctx.APIURL, ctx.AgentUserID, Purpose)
	tokens := fetchTokensFrom(flow, ctx.InternalClient(), endpoint)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no Google Drive tokens available — connect a Google account with Drive permissions")
	}
	return tokens, nil
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
//
// Errored tokens (Launch couldn't refresh them) are intentionally
// dropped here — the caller is expected to use SelectActiveOrError
// instead when it needs to surface a useful failure message rather
// than the generic "no accounts available" line.
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

// SelectActive splits a token slice into usable and errored buckets,
// respecting accountFilter. Use this when you want to render a human-
// readable failure ("re-link foo@bar.com — refresh failed: ...") for
// the AI tool result rather than silently dropping the dead account
// and returning "no Google accounts available".
//
// Returned errors carry the full Launch-side error string verbatim;
// they're the only place the user gets to see what Google actually
// said about their refresh token.
func SelectActive(tokens []TokenInfo, accountFilter string) (active []TokenInfo, errored []TokenInfo) {
	for _, t := range tokens {
		matchesFilter := accountFilter == "" ||
			strings.EqualFold(t.Email, accountFilter) ||
			strings.EqualFold(t.Label, accountFilter) ||
			strings.Contains(strings.ToLower(t.Email), strings.ToLower(accountFilter))
		if !matchesFilter {
			continue
		}
		if t.Error != "" {
			errored = append(errored, t)
			continue
		}
		active = append(active, t)
	}
	return active, errored
}

// FormatTokenErrors renders an errored-token list as a single
// user-facing string. Used by actions to surface refresh failures
// in tool results so the agent can tell the user exactly which
// account needs re-linking and why.
func FormatTokenErrors(errored []TokenInfo) string {
	if len(errored) == 0 {
		return ""
	}
	parts := make([]string, 0, len(errored))
	for _, t := range errored {
		label := t.Email
		if label == "" {
			label = t.Label
		}
		if label == "" {
			label = "Google account"
		}
		parts = append(parts, fmt.Sprintf("%s: %s", label, t.Error))
	}
	return "Google account refresh failed — please re-link affected account(s). Details: " + strings.Join(parts, "; ")
}

// DoRequest makes an authenticated HTTP request to a Google API.
func DoRequest(flow *core.Flow, method, url, accessToken string, body []byte) (int, []byte, error) {
	return doRequestWithTimeout(flow, method, url, accessToken, body, defaultTimeout)
}

// DoRequestLong makes an authenticated HTTP request with a longer timeout,
// suitable for file uploads and downloads.
func DoRequestLong(flow *core.Flow, method, url, accessToken string, body []byte) (int, []byte, error) {
	return doRequestWithTimeout(flow, method, url, accessToken, body, longTimeout)
}

func doRequestWithTimeout(flow *core.Flow, method, url, accessToken string, body []byte, timeout time.Duration) (int, []byte, error) {
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

// DoMultipartUpload performs a multipart/related upload to the Google Drive API.
func DoMultipartUpload(flow *core.Flow, url, accessToken string, metadata []byte, content []byte, contentType string) (int, []byte, error) {
	boundary := "flomation_upload_boundary"
	var buf bytes.Buffer
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: application/json; charset=UTF-8\r\n\r\n")
	buf.Write(metadata)
	buf.WriteString("\r\n--" + boundary + "\r\n")
	buf.WriteString("Content-Type: " + contentType + "\r\n\r\n")
	buf.Write(content)
	buf.WriteString("\r\n--" + boundary + "--\r\n")

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, &buf)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "multipart/related; boundary="+boundary)

	client := &http.Client{Timeout: longTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("upload request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, respBody, nil
}

// HandleAuthError disconnects the Google account if the response indicates
// expired or revoked credentials. Fire-and-forget.
func HandleAuthError(flow *core.Flow, email string, statusCode int) {
	if statusCode != http.StatusUnauthorized && statusCode != http.StatusForbidden {
		return
	}
	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" || ctx.AgentUserID == "" {
		return
	}
	endpoint := fmt.Sprintf("%s/api/v1/internal/agent-user/%s/google-account/%s?purpose=%s",
		ctx.APIURL, ctx.AgentUserID, email, Purpose)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodDelete, endpoint, nil)
	if err != nil {
		return
	}
	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		log.WithError(err).Warn("[google] failed to disconnect expired account")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	log.WithFields(log.Fields{
		"email":  email,
		"status": resp.StatusCode,
	}).Info("[google] disconnected expired account")
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
	if c == nil || c.Number() == nil {
		return defaultVal
	}
	v := int(*c.Number())
	if v <= 0 {
		return defaultVal
	}
	return v
}

// OptBool extracts an optional boolean input value.
func OptBool(name string, inputs []*core.Connection) bool {
	return OptStr(name, inputs) == "true"
}

// AnchorAppendRange anchors a bare sheet-name range to cell A1 so Google's
// append aligns new rows to column A. Google's append searches the given
// range for a "logical table" and writes from that table's first column;
// with a bare sheet name the whole sheet is searched, so empty leading
// columns (or a stray far-column block) can make it start at, say, column
// L. Anchoring to A1 fixes the common header-at-A1 case. A range that
// already names a cell/range (contains "!") is respected and returned
// unchanged, so a caller can still target a deliberate non-A anchor.
func AnchorAppendRange(r string) string {
	if r == "" || strings.Contains(r, "!") {
		return r
	}
	return r + "!A1"
}

// ProtectPhoneLikeText guards phone/ID-shaped string cells against Google
// Sheets' USER_ENTERED coercion. In that mode Sheets parses each value as
// if a human typed it into the cell, so "07700900123" loses its leading
// zero (→ 7700900123) and "+447700900123" is evaluated as a number
// (→ 447700900123). Prefixing such a value with an apostrophe forces Sheets
// to store it verbatim as text — the apostrophe itself is not displayed.
//
// Only string cells whose shape Sheets would corrupt are touched:
//   - a run of digits with a leading zero (phone, sort code, postcode…),
//   - an international number: a leading "+" followed by digits.
// Common separators (spaces, dashes, dots, parentheses) are ignored when
// testing the shape. Numbers, dates, formulas ("=…"), negatives and
// ordinary text pass through untouched. Mutates values in place.
//
// Only call this for the USER_ENTERED path — under RAW, Sheets already
// stores values verbatim and a leading apostrophe would be written
// literally into the cell.
func ProtectPhoneLikeText(values [][]interface{}) {
	for i := range values {
		for j := range values[i] {
			if s, ok := values[i][j].(string); ok && phoneLikeNeedsTextGuard(s) {
				values[i][j] = "'" + s
			}
		}
	}
}

// phoneLikeNeedsTextGuard reports whether a string would be corrupted by
// Sheets' USER_ENTERED numeric coercion and should be stored as text.
func phoneLikeNeedsTextGuard(s string) bool {
	// Strip common phone separators before testing the numeric shape.
	compact := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "").Replace(s)
	if compact == "" {
		return false
	}
	// International: a leading "+" then all digits.
	if compact[0] == '+' {
		return len(compact) > 1 && isAllDigits(compact[1:])
	}
	// Leading-zero numeric run (phone, sort code, postcode, account…).
	if compact[0] == '0' && len(compact) >= 2 && isAllDigits(compact) {
		return true
	}
	return false
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ─── Drive shared-files helpers ────────────────────────────────────
//
// Google Drive's files.list / files.get / files.update endpoints
// default to "user's My Drive only" — silently dropping any file
// that's been shared with the user but lives outside their own
// drive. The fix is two-or-three parameters that opt into the
// broader corpus:
//
//   - supportsAllDrives=true            (REQUIRED for any single-
//                                        file op that could touch
//                                        a shared drive)
//   - includeItemsFromAllDrives=true    (only for list/search —
//                                        actually shows the shared
//                                        items in the response)
//   - corpora=allDrives                 (only for list/search —
//                                        widens the scope from
//                                        "user" to "everything")
//
// AppendDriveSingleFileParams adds the minimum needed for any
// operation that targets a specific file ID (get, download, copy,
// delete, move, share, etc.) — just supportsAllDrives.
//
// AppendDriveListParams adds the full triple for list / search
// queries, accepting a "corpora" hint so the caller can narrow
// (e.g. to just shared-with-me) when needed.

// AppendDriveSingleFileParams adds the supportsAllDrives flag to
// a query string. Use on any endpoint that references a specific
// file ID. Safe to call multiple times — the flag is idempotent.
func AppendDriveSingleFileParams(params url.Values) {
	params.Set("supportsAllDrives", "true")
}

// AppendDriveListParams adds the three params needed for a list
// or search query to return files outside the user's My Drive.
// Pass "" for corpora to use the default "allDrives" — the widest
// possible scope.
func AppendDriveListParams(params url.Values, corpora string) {
	params.Set("supportsAllDrives", "true")
	params.Set("includeItemsFromAllDrives", "true")
	if corpora == "" {
		corpora = "allDrives"
	}
	params.Set("corpora", corpora)
}

// AppendDriveSingleFileQS appends ?supportsAllDrives=true (or
// &supportsAllDrives=true if the URL already has a query string)
// to a URL string built without url.Values. Used by call sites
// that construct their URL inline rather than via url.Values.
func AppendDriveSingleFileQS(rawURL string) string {
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return rawURL + sep + "supportsAllDrives=true"
}

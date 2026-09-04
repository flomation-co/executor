// Package freshworks_common holds the transport shared by every Freshworks
// product — Freshsales today, Freshdesk and Freshservice if they follow.
//
// What is genuinely common across the suite is the shape of the thing: a
// per-tenant subdomain under a single parent domain, a key pasted by the
// operator, and a "Token token=" authorisation header. Product-specific
// concerns (the API path prefix, entity helpers, result shapes) live in the
// product's own common package and NOT here.
//
// No Execute function, so the manifest generator skips this package.
package freshworks_common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

// HostSuffix is the only domain a Freshworks bundle can legitimately live on.
//
// SECURITY-CRITICAL. The bundle alias is operator-supplied and is used to build
// every request URL, with the customer's API key attached. Left unvalidated, a
// crafted alias sends that key to an arbitrary host on the first run of the
// flow. Same hazard, same guard, and the same reasoning as
// salesforce_common's instanceURL suffix list — Freshworks is simply the easier
// case, because there is exactly one legitimate suffix.
const HostSuffix = ".myfreshworks.com"

// requestTimeout bounds a single call. Freshworks is a normal REST API; a
// minute is generous for the bulk endpoints and short enough that a hung
// request does not pin an execution.
const requestTimeout = 60 * time.Second

// maxResponseBytes caps what we will read back, so a pathological response
// cannot exhaust the executor's memory.
const maxResponseBytes = 24 << 20

// testBaseURL, when set, replaces the operator's bundle for every request AND
// relaxes host validation, so action packages can exercise Execute end-to-end
// against an httptest server. Test-only; the same seam idiom as
// salesforce_common.SetHostForTest.
var testBaseURL string

// SetHostForTest redirects every request to the given base URL and returns a
// function restoring real behaviour. Test-only.
func SetHostForTest(base string) func() {
	prev := testBaseURL
	testBaseURL = strings.TrimRight(base, "/")
	return func() { testBaseURL = prev }
}

// NormaliseBundle reduces whatever the operator pasted to a bare origin.
//
// Accepts the bare alias people actually know ("widgetz"), the full host
// ("widgetz.myfreshworks.com"), a complete URL, and a URL carrying a path or a
// trailing slash. Returns "https://widgetz.myfreshworks.com".
//
// Deliberately does NOT validate — ValidateBundle does that — so callers can
// normalise and then report on the cleaned value.
func NormaliseBundle(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}

	// A bare alias with no dots and no scheme is the common case.
	if !strings.Contains(s, ".") && !strings.Contains(s, "/") && !strings.Contains(s, ":") {
		return "https://" + strings.ToLower(s) + HostSuffix
	}

	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return ""
	}
	return "https://" + strings.ToLower(u.Host)
}

// ValidateBundle confirms a normalised origin points at Freshworks and nowhere
// else. Returns the origin on success.
//
// Rejects a bare "myfreshworks.com" with no alias as well as any foreign host:
// the suffix check alone would pass "evil-myfreshworks.com", so the host must
// end in ".myfreshworks.com" AND have something in front of it.
func ValidateBundle(raw string) (string, error) {
	if testBaseURL != "" {
		return testBaseURL, nil
	}

	origin := NormaliseBundle(raw)
	if origin == "" {
		return "", fmt.Errorf("account is required — your Freshworks bundle alias, e.g. \"widgetz\" or \"widgetz%s\"", HostSuffix)
	}

	host := strings.TrimPrefix(origin, "https://")
	if !strings.HasSuffix(host, HostSuffix) || len(host) <= len(HostSuffix) {
		return "", fmt.Errorf("%q is not a Freshworks account — the address must end in %s", host, HostSuffix)
	}
	return origin, nil
}

// Client is a per-call Freshworks HTTP client.
//
// Built per call and never a package-level global: the executor runs many
// tenants' flows in one process, so a shared client carrying one customer's key
// or host would leak across them. Same reasoning as
// stripe_common.NewClient and apollo_common.NewClient.
type Client struct {
	origin string
	apiKey string
	prefix string
	http   *http.Client
}

// NewClient binds a validated origin, a key and a product path prefix
// (Freshsales: "/crm/sales/api") into one caller.
func NewClient(origin, apiKey, prefix string) *Client {
	return &Client{
		origin: strings.TrimRight(origin, "/"),
		apiKey: apiKey,
		prefix: "/" + strings.Trim(prefix, "/"),
		http:   &http.Client{Timeout: requestTimeout},
	}
}

// URL builds an absolute request URL for a product-relative path.
func (c *Client) URL(path string) string {
	return c.origin + c.prefix + "/" + strings.TrimLeft(path, "/")
}

// APIError carries a Freshworks failure with enough detail for a flow author to
// act on it. Status is kept so callers can special-case 429.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return e.Message }

// Do performs a request and decodes the JSON response.
//
// A nil body sends no payload, which is what GET and DELETE want. The
// execution's context is threaded through so a cancelled flow does not leave a
// request in flight.
func (c *Client) Do(flow *core.Flow, method, path string, body interface{}, query url.Values) (map[string]interface{}, error) {
	target := c.URL(path)
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("could not encode the request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(goContext(flow), method, target, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token token="+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach Freshworks: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("could not read the response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Status: resp.StatusCode, Message: describeFailure(resp.StatusCode, raw)}
	}

	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]interface{}{}, nil
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		// A 2xx that is not a JSON object is still a success; hand back the
		// text rather than failing a call that worked.
		return map[string]interface{}{"response": string(raw)}, nil
	}
	return decoded, nil
}

// describeFailure turns a non-2xx into something a flow author can act on.
//
// The rate limit gets its own sentence because Freshworks allows only 1000
// requests per hour per account — low enough that hitting it is a normal
// operational event rather than a bug, and the generic "429" tells nobody what
// to do about it.
func describeFailure(status int, raw []byte) string {
	detail := extractMessage(raw)

	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		if detail == "" {
			detail = "the API key was rejected"
		}
		return fmt.Sprintf("Freshworks refused the request (%d): %s. Check the API key in Profile Settings ▸ API Settings, and that it belongs to this account.", status, detail)
	case http.StatusNotFound:
		if detail == "" {
			detail = "not found"
		}
		return fmt.Sprintf("Freshworks could not find that record (404): %s", detail)
	case http.StatusTooManyRequests:
		return "Freshworks rate limit reached (429). The limit is 1000 requests per hour per account — wait, or reduce how often this flow runs."
	}

	if detail == "" {
		detail = strings.TrimSpace(string(raw))
	}
	if detail == "" {
		detail = "no detail returned"
	}
	return fmt.Sprintf("Freshworks error (%d): %s", status, truncate(detail, 400))
}

// extractMessage digs the human-readable part out of a Freshworks error body.
// The shape varies by endpoint, so several are tried before giving up.
func extractMessage(raw []byte) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}

	if errs, ok := payload["errors"].(map[string]interface{}); ok {
		if msg, ok := errs["message"].(string); ok && msg != "" {
			return msg
		}
		if list, ok := errs["message"].([]interface{}); ok && len(list) > 0 {
			parts := make([]string, 0, len(list))
			for _, v := range list {
				parts = append(parts, fmt.Sprintf("%v", v))
			}
			return strings.Join(parts, "; ")
		}
		// Field-level validation: {"errors":{"email":["is invalid"]}}
		parts := []string{}
		for field, v := range errs {
			parts = append(parts, fmt.Sprintf("%s %v", field, v))
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	for _, key := range []string{"message", "error", "description"} {
		if msg, ok := payload[key].(string); ok && msg != "" {
			return msg
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// goContext returns the execution's context so a cancelled flow does not leave
// a request in flight.
//
// With no flow (an action exercised directly in a test) the background context
// is enough: the http.Client already carries requestTimeout, so wrapping a
// timeout context around it would bound nothing extra while leaking its timer.
func goContext(flow *core.Flow) context.Context {
	if flow != nil {
		return flow.GoContext()
	}
	return context.Background()
}

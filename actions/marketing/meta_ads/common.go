// Package meta_ads_common holds the shared Graph API client, auth inputs and
// result shapers for every Meta Ads action. It has no Execute function, so the
// manifest generator excludes it from the action registry.
//
// The Marketing API is not a separate service — it is the same Graph API host
// and the same token model as social/facebook, with a different set of edges
// and permissions (ads_read to read, ads_management to write). See
// PLAN-meta-ads.md for the access-approval path, which is the slow part.
//
// This deliberately does NOT import social/facebook's client. The two need to
// move between Graph versions independently, and coupling them would mean a
// version bump for one silently changing the other's request surface.
package meta_ads_common

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
)

// BaseURL is the Graph API root. A var so tests can point it at an httptest
// server.
//
// Pinned to v25.0 rather than the newest (v26.0, released 29 July 2026): v25.0
// runs until July 2028 and is what the current Insights documentation is
// written against. Meta expires a version roughly two years after release and
// calls to an expired version stop behaving predictably, so the version is a
// deliberate, dated decision rather than "whatever was current when written".
var BaseURL = "https://graph.facebook.com/v25.0"

const (
	requestTimeout = 60 * time.Second
	maxResponse    = 8 << 20 // 8 MB — Insights responses are large
)

// AuthInputs is the credential pair every Meta Ads action embeds first.
//
// app_secret is optional but strongly recommended: with "Require App Secret"
// enabled on the app, Meta rejects any call without an appsecret_proof HMAC.
var AuthInputs = []core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
}

// Client is a Graph API client scoped to one token. Built per call — the
// executor runs many tenants' flows concurrently, so a package-level client
// would leak one tenant's credentials into another's request.
type Client struct {
	accessToken string
	appSecret   string
	http        *http.Client
}

func NewClient(accessToken, appSecret string) *Client {
	return &Client{accessToken: accessToken, appSecret: appSecret, http: &http.Client{Timeout: requestTimeout}}
}

// Get performs a GET against a Graph edge and returns the decoded JSON.
func (c *Client) Get(flow *core.Flow, path string, params url.Values) (map[string]interface{}, error) {
	return c.do(flow, http.MethodGet, path, params)
}

// Post performs a POST. Graph writes are form-encoded, not JSON.
func (c *Client) Post(flow *core.Flow, path string, params url.Values) (map[string]interface{}, error) {
	return c.do(flow, http.MethodPost, path, params)
}

func (c *Client) do(flow *core.Flow, method, path string, params url.Values) (map[string]interface{}, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("access_token", c.accessToken)

	// appsecret_proof is an HMAC-SHA256 of the access token keyed by the app
	// secret. Required when the app has "Require App Secret" turned on, and
	// harmless otherwise, so it is always sent when a secret is supplied.
	if c.appSecret != "" {
		mac := hmac.New(sha256.New, []byte(c.appSecret))
		mac.Write([]byte(c.accessToken))
		params.Set("appsecret_proof", hex.EncodeToString(mac.Sum(nil)))
	}

	endpoint := BaseURL + path
	var req *http.Request
	var err error
	if method == http.MethodGet {
		req, err = http.NewRequest(method, endpoint+"?"+params.Encode(), nil)
	} else {
		req, err = http.NewRequest(method, endpoint, strings.NewReader(params.Encode()))
	}
	if err != nil {
		return nil, err
	}
	if flow != nil {
		req = req.WithContext(flow.GoContext())
	}
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to the Meta Marketing API failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var decoded map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &decoded); err != nil {
			return nil, fmt.Errorf("unable to parse Marketing API response: %w", err)
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiError(resp.StatusCode, decoded, body)
	}
	return decoded, nil
}

// apiError turns Graph's error envelope into something a flow author can act
// on. Meta's messages are usually specific ("Invalid parameter"), so the
// user-facing detail — error_user_msg — is preferred where present, and the
// subcode is kept because that is what distinguishes a rate limit from a
// permissions failure.
func apiError(status int, decoded map[string]interface{}, raw []byte) error {
	e, _ := decoded["error"].(map[string]interface{})
	if e == nil {
		return fmt.Errorf("error from the Meta Marketing API (HTTP %d): %s", status, strings.TrimSpace(string(raw)))
	}

	msg := str(e, "error_user_msg")
	if msg == "" {
		msg = str(e, "message")
	}
	code := num(e, "code")
	sub := num(e, "error_subcode")
	blame := blameDetail(e)

	// 4 / 17 / 32 / 613 are the throttling family. Say so plainly: on the
	// Limited access tier these are expected rather than exceptional, and a
	// flow author needs to know the difference between "you are going too fast"
	// and "your request is wrong".
	switch code {
	case 4, 17, 32, 613:
		return fmt.Errorf("rate limit reached on the Meta Marketing API (code %d): %s — the app's Marketing API tier may still be Limited Access, which is heavily rate-limited per ad account. See PLAN-meta-ads.md", code, msg)
	case 190:
		return fmt.Errorf("the Meta access token is invalid or expired (code 190): %s — a user token expires with the person's session; use a System User token for anything long-lived", msg)
	case 200, 10:
		return fmt.Errorf("permissions error from Meta (code %d, subcode %d): %s — the token needs ads_management (write) or ads_read (read), and the app must have access to this ad account", code, sub, msg)
	}
	if sub != 0 {
		return fmt.Errorf("error from the Meta Marketing API (code %d, subcode %d): %s%s", code, sub, msg, blame)
	}
	return fmt.Errorf("error from the Meta Marketing API (code %d): %s%s", code, msg, blame)
}

// --- ad account id handling ---

// AccountPath normalises an ad account id into its edge path.
//
// Meta ad account ids are written both ways in the wild: Ads Manager shows a
// bare number, while the API needs the act_ prefix. Accepting either removes a
// class of "Unsupported get request" failures that read as a permissions
// problem rather than a formatting one.
func AccountPath(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if !strings.HasPrefix(id, "act_") {
		id = "act_" + id
	}
	return "/" + id
}

// --- money ---

// AccountCurrency reads an ad account's currency (e.g. "GBP").
//
// Budgets go to Meta as integers in the ad account's currency MINOR unit, so
// the exponent has to be right: 50 sent with the wrong exponent is not a
// rounding error, it is a 100x overspend on a live campaign. The currency is
// therefore fetched from the account rather than taken as an input, because an
// input can disagree with reality and nothing would catch it until the money
// had been spent.
func AccountCurrency(flow *core.Flow, c *Client, accountID string) (string, error) {
	resp, err := c.Get(flow, AccountPath(accountID), url.Values{"fields": {"currency"}})
	if err != nil {
		return "", fmt.Errorf("could not read the ad account's currency: %w", err)
	}
	cur := str(resp, "currency")
	if cur == "" {
		return "", fmt.Errorf("the ad account did not report a currency, so a budget cannot be converted safely")
	}
	return cur, nil
}

// BudgetMinorUnits converts a ConnectionTypeMoney input (major units, e.g.
// "50.00") into the integer minor-unit value Meta expects, using the account's
// own currency for the exponent.
//
// Returns nil when the input is blank, so an unset budget stays unset rather
// than becoming zero — a zero budget is a validation error at Meta, but a
// silently-omitted one is what "I did not set this" should mean.
func BudgetMinorUnits(name, currency string, inputs []*core.Connection) (*int64, error) {
	// Reuses the Stripe helper rather than reimplementing currency exponents:
	// the 0-decimal (JPY, KRW) and 3-decimal (KWD, BHD) cases are exactly the
	// sort of table that rots when duplicated. It lives under actions/stripe
	// only because that is where money was first needed; it has no Execute, so
	// it is a plain helper package, not an action. Worth promoting to a shared
	// money package when a third caller appears.
	return stripe_common.MoneyToMinorUnits(name, currency, inputs)
}

// --- input helpers ---

func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return strings.TrimSpace(*c.String())
}

// GetAuth reads the token pair, refusing an unresolved ${...} reference rather
// than sending it as a literal token and getting an opaque 190 back.
func GetAuth(inputs []*core.Connection) (token, secret string, err error) {
	token, err = RequiredString("access_token", inputs)
	if err != nil {
		return "", "", fmt.Errorf("a Meta access token is required")
	}
	if strings.HasPrefix(token, "${") {
		return "", "", fmt.Errorf("the Meta access token did not resolve — set an environment secret and reference it as ${secrets.X}")
	}
	return token, OptionalString("app_secret", inputs), nil
}

// SetParam assigns a string input when present.
func SetParam(p url.Values, key, name string, inputs []*core.Connection) {
	if v := OptionalString(name, inputs); v != "" {
		p.Set(key, v)
	}
}

// SetJSONParam assigns a JSON-object input verbatim after validating it parses.
// Graph takes several fields (targeting, promoted_object) as JSON-encoded
// strings inside a form-encoded body, so invalid JSON must be caught here
// rather than surfacing as a generic "Invalid parameter".
func SetJSONParam(p url.Values, key, name string, inputs []*core.Connection) error {
	v := OptionalString(name, inputs)
	if v == "" {
		return nil
	}
	var probe interface{}
	if err := json.Unmarshal([]byte(v), &probe); err != nil {
		return fmt.Errorf("%s must be valid JSON: %w", name, err)
	}
	p.Set(key, v)
	return nil
}

// MergeJSONFields flattens a JSON-object input into the request parameters, so
// an author can reach any Graph field the action does not model explicitly
// without waiting for it to be added.
//
// Scalars go in as their string form; nested objects and arrays are
// re-marshalled to JSON, because Graph takes structured values as JSON-encoded
// STRINGS inside a form-encoded body rather than as nested form keys.
//
// Curated inputs are applied first and this runs last, so an explicit override
// wins over the action's own defaults — which is the only useful precedence for
// an escape hatch.
func MergeJSONFields(p url.Values, name string, inputs []*core.Connection) error {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return fmt.Errorf("%s must be a JSON object: %w", name, err)
	}
	for k, v := range obj {
		switch vv := v.(type) {
		case nil:
			continue
		case string:
			p.Set(k, vv)
		case bool:
			p.Set(k, strconv.FormatBool(vv))
		case float64:
			// Graph ids and budgets are integers; %v on a float64 would render
			// 1234567890123 as 1.234567890123e+12 and be rejected.
			if vv == float64(int64(vv)) {
				p.Set(k, strconv.FormatInt(int64(vv), 10))
			} else {
				p.Set(k, strconv.FormatFloat(vv, 'f', -1, 64))
			}
		default:
			b, err := json.Marshal(vv)
			if err != nil {
				return fmt.Errorf("%s.%s could not be encoded: %w", name, k, err)
			}
			p.Set(k, string(b))
		}
	}
	return nil
}

// Fields joins a comma-separated field list, falling back to a default set.
func Fields(name string, inputs []*core.Connection, fallback string) string {
	if v := OptionalString(name, inputs); v != "" {
		return v
	}
	return fallback
}

// --- result shapers ---

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{"tool_result": msg, "success": false, "error": msg}
}

func OkResult(summary string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{"tool_result": summary, "success": true, "error": ""}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// ListResult embeds the records in tool_result, not just a count — an AI caller
// that gets "Found 12 campaigns" and nothing else has to make a second call to
// do anything useful.
func ListResult(items []map[string]interface{}, summary string, extra map[string]interface{}) map[string]interface{} {
	if items == nil {
		items = []map[string]interface{}{}
	}
	body := summary
	if b, err := json.Marshal(items); err == nil && len(b) > 2 {
		body = summary + ":\n" + string(b)
	}
	out := map[string]interface{}{
		"tool_result": body,
		"results":     items,
		"count":       len(items),
		"success":     true,
		"error":       "",
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// Data pulls the "data" array out of a Graph list response.
func Data(resp map[string]interface{}) []map[string]interface{} {
	arr, ok := resp["data"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

// NextCursor returns the paging cursor for the next page, empty when the list
// is exhausted. Surfaced as an output so a flow can loop rather than silently
// processing only the first page.
func NextCursor(resp map[string]interface{}) string {
	paging, _ := resp["paging"].(map[string]interface{})
	if paging == nil {
		return ""
	}
	// A `next` URL is present only when there IS another page, so its absence
	// is the end-of-list signal even though cursors.after may still be set.
	if _, ok := paging["next"].(string); !ok {
		return ""
	}
	cursors, _ := paging["cursors"].(map[string]interface{})
	return str(cursors, "after")
}

func str(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func num(m map[string]interface{}, key string) int64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

// --- customer-list audience hashing ---

// AudienceSchema is a customer-list field Meta accepts for matching.
type AudienceSchema string

const (
	SchemaEmail AudienceSchema = "EMAIL"
	SchemaPhone AudienceSchema = "PHONE"
)

// HashAudienceValue normalises a customer-list value and returns its SHA-256
// hex digest.
//
// Raw personal data must NEVER reach Meta: customer-list audiences are matched
// on hashes precisely so the advertiser does not hand over their list in the
// clear. Sending an unhashed email would leak a customer's identity to a third
// party for no benefit, since Meta would fail to match it anyway.
//
// Normalisation is not optional either — it IS the matching key. Meta hashes
// its own side after the same normalisation, so " Ada@Example.COM " and
// "ada@example.com" must produce the same digest or the match silently fails
// and the audience quietly under-counts with nothing to indicate why.
//
//   - EMAIL: trim, lowercase.
//   - PHONE: digits only (drop +, spaces, brackets, hyphens), keeping the
//     country code, i.e. E.164 without the plus.
//
// An empty or unusable value returns ok=false so the caller can skip it rather
// than send the hash of an empty string, which matches nobody and inflates the
// reported upload count.
func HashAudienceValue(schema AudienceSchema, raw string) (string, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", false
	}

	switch schema {
	case SchemaEmail:
		v = strings.ToLower(v)
		// Hashing something that is not an address produces a digest that
		// cannot match, and counting it as uploaded would overstate the
		// audience. Require a local part, a single @, and a dotted domain.
		local, domain, found := strings.Cut(v, "@")
		if !found || local == "" || domain == "" ||
			strings.Contains(domain, "@") || !strings.Contains(domain, ".") ||
			strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
			return "", false
		}
	case SchemaPhone:
		// "(0)" is the European convention for an optional trunk prefix —
		// "+44 (0)7700 900123" means 447700900123, NOT 4407700900123. Stripping
		// only the digits would keep that zero and produce a number that
		// silently matches nobody, which is exactly the failure this whole
		// function exists to avoid. The brackets mark the zero unambiguously,
		// so removing it is safe; guessing at unbracketed trunk zeros is not,
		// and is deliberately not attempted.
		v = strings.ReplaceAll(v, "(0)", "")
		var digits strings.Builder
		for _, r := range v {
			if r >= '0' && r <= '9' {
				digits.WriteRune(r)
			}
		}
		v = digits.String()
		// Too short to carry a country code plus a subscriber number.
		//
		// Known limit: a number in NATIONAL format ("07700900123") passes this
		// check but will not match, because Meta matches on country code. There
		// is no safe way to infer the country here, so the caller has to supply
		// numbers in international form.
		if len(v) < 7 {
			return "", false
		}
	default:
		return "", false
	}

	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:]), true
}

// BuildAudiencePayload hashes every supplied value and returns Meta's
// customer-list payload, plus how many values were usable and how many were
// skipped.
//
// The skipped count is returned rather than swallowed: an upload that silently
// drops half its list looks identical to one that worked.
func BuildAudiencePayload(schema AudienceSchema, values []string) (payload string, used, skipped int, err error) {
	data := make([][]string, 0, len(values))
	for _, v := range values {
		h, ok := HashAudienceValue(schema, v)
		if !ok {
			skipped++
			continue
		}
		data = append(data, []string{h})
	}
	if len(data) == 0 {
		return "", 0, skipped, fmt.Errorf("no usable %s values — all %d entries were blank or malformed", schema, skipped)
	}

	b, err := json.Marshal(map[string]interface{}{
		"schema": []string{string(schema)},
		"data":   data,
	})
	if err != nil {
		return "", 0, skipped, err
	}
	return string(b), len(data), skipped, nil
}

// SplitLines splits a newline- or comma-separated list into trimmed values.
func SplitLines(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// SetJSONValue marshals a Go value into a parameter as a JSON string, which is
// how Graph accepts structured fields inside a form-encoded body.
func SetJSONValue(p url.Values, key string, value interface{}) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("could not encode %s: %w", key, err)
	}
	p.Set(key, string(b))
	return nil
}

// DescribeBudget renders what was ACTUALLY sent to Meta for a money field, in
// both units: "daily budget 10.00 GBP (sent as 1000)".
//
// This exists because the dangerous failure here is silent and hundredfold.
// Meta takes budgets as minor-unit integers, so an AI that knows the API — and
// helpfully pre-converts £10 to 1000 before calling — gets 1000 multiplied
// again into 100000, and books £1,000 a day instead of £10. Nothing errors; the
// campaign is simply created wrong, and reading it back shows a large number
// that is easy to misread a second time.
//
// Echoing both the major-unit input and the exact integer sent makes that
// visible in the tool result at a glance, to a human or an agent, without
// anyone having to know the convention.
func DescribeBudget(label, majorInput, currency string, minorSent int64) string {
	if strings.TrimSpace(majorInput) == "" {
		return ""
	}
	return fmt.Sprintf("%s %s %s (sent to Meta as %d)", label, majorInput, currency, minorSent)
}

// blameDetail extracts the field-level detail Meta buries in error_data, and
// renders it as a suffix.
//
// Meta routinely answers a malformed request with a sentence that names nothing
// — "The ad creative is invalid" — while separately reporting exactly WHICH
// field it objected to, in error_data.blame_field_specs. Dropping that turns a
// precise answer into a guessing game: the reader is left choosing between
// permissions, an unapproved image and a provisioning delay, when Meta already
// said which field was wrong.
//
// blame_field_specs is an array of field PATHS, each itself an array of
// segments, so it arrives as [["creative","object_story_spec","link_data",
// "image_hash"]] and has to be flattened to be readable.
func blameDetail(e map[string]interface{}) string {
	data, _ := e["error_data"].(map[string]interface{})
	if data == nil {
		return ""
	}

	var parts []string
	if specs, ok := data["blame_field_specs"].([]interface{}); ok {
		for _, spec := range specs {
			segs, ok := spec.([]interface{})
			if !ok {
				// Occasionally a bare string rather than a path array.
				if s, ok := spec.(string); ok && strings.TrimSpace(s) != "" {
					parts = append(parts, s)
				}
				continue
			}
			var path []string
			for _, seg := range segs {
				if s, ok := seg.(string); ok && s != "" {
					path = append(path, s)
				}
			}
			if len(path) > 0 {
				parts = append(parts, strings.Join(path, "."))
			}
		}
	}

	// Some errors carry a plain-text detail instead of, or as well as, the
	// field paths.
	if detail := str(data, "blame_field"); detail != "" {
		parts = append(parts, detail)
	}

	if len(parts) == 0 {
		return ""
	}
	return " — Meta names the offending field(s): " + strings.Join(parts, ", ")
}

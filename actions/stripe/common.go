// Package stripe_common holds the shared client, auth input, helpers and
// result shapers for every Stripe action. It has no Execute function, so the
// manifest generator excludes it from the action registry.
package stripe_common

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	stripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
)

// NewClient builds a Stripe API client scoped to a single tenant's secret key.
//
// CRITICAL: the executor runs many tenants' flows concurrently, so we must
// NEVER set the package-level stripe.Key (that would race across flows and
// could send one tenant's request with another's key). Each action gets its
// own *client.API bound to its own key via Init.
func NewClient(apiKey string) *client.API {
	sc := &client.API{}
	sc.Init(apiKey, nil)
	return sc
}

// AuthInputs is the shared credential input (Stripe secret key). Supplied by
// the flow author as an environment secret (${secrets.X}).
var AuthInputs = []core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "Stripe Secret Key",
		Placeholder: "sk_live_… or sk_test_…",
		Required:    true,
	},
}

// --- input helpers ---

func GetAPIKey(inputs []*core.Connection) (string, error) {
	return RequiredString("api_key", inputs)
}

func RequiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || *c.String() == "" {
		return "", &missingInputError{name}
	}
	return *c.String(), nil
}

func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func OptionalInt64(name string, inputs []*core.Connection) *int64 {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	return c.Number()
}

func OptionalBool(name string, inputs []*core.Connection) *bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	return c.Boolean()
}

// Metadata reads a "metadata" KeyValueArray input into a map for
// params.Metadata. Returns nil when absent so it's omitted from the request.
func Metadata(inputs []*core.Connection) map[string]string {
	c := core.FindConnection("metadata", inputs)
	if c == nil {
		return nil
	}
	pairs := c.KeyValuePairs()
	if len(pairs) == 0 {
		return nil
	}
	m := make(map[string]string, len(pairs))
	for _, kv := range pairs {
		if kv.Key != "" {
			m[kv.Key] = kv.Value
		}
	}
	return m
}

// CSVToList splits a comma-separated input (e.g. expand fields) into a slice.
func CSVToList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// IdempotencyKey reads an optional idempotency_key input; empty means none.
func IdempotencyKey(inputs []*core.Connection) string {
	return OptionalString("idempotency_key", inputs)
}

// --- money handling ---

// zeroDecimalCurrencies have no minor unit — the amount IS the major value
// (e.g. JPY 1000 = ¥1000, not 100000). Source: Stripe zero-decimal list.
var zeroDecimalCurrencies = map[string]bool{
	"bif": true, "clp": true, "djf": true, "gnf": true, "jpy": true,
	"kmf": true, "krw": true, "mga": true, "pyg": true, "rwf": true,
	"ugx": true, "vnd": true, "vuv": true, "xaf": true, "xof": true, "xpf": true,
}

// threeDecimalCurrencies use 1/1000 of the major unit (e.g. BHD, KWD).
var threeDecimalCurrencies = map[string]bool{
	"bhd": true, "jod": true, "kwd": true, "omr": true, "tnd": true,
}

// currencyExponent returns the number of decimal places for a currency's
// smallest unit (2 for most, 0 for zero-decimal, 3 for three-decimal).
func currencyExponent(currency string) int {
	c := strings.ToLower(strings.TrimSpace(currency))
	switch {
	case zeroDecimalCurrencies[c]:
		return 0
	case threeDecimalCurrencies[c]:
		return 3
	default:
		return 2
	}
}

// MoneyToMinorUnits reads a money input (a decimal in MAJOR units, e.g. "12.34"
// or "£12.34") and converts it to the currency's smallest unit for the Stripe
// API. Returns (nil, nil) when the input is absent/blank so the field is simply
// omitted from the request; returns an error on an unparseable amount so the
// action can surface a graceful ErrorResult rather than silently sending 0.
func MoneyToMinorUnits(name, currency string, inputs []*core.Connection) (*int64, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}

	// Strip common currency symbols, thousands separators and whitespace so
	// "£1,234.50" and "1234.50" both parse.
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case '£', '$', '€', '¥', ',', ' ', '\t':
			return -1
		}
		return r
	}, raw)

	major, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount %q: expected a number like 12.34", raw)
	}
	if major < 0 {
		return nil, fmt.Errorf("invalid amount %q: must not be negative", raw)
	}

	factor := math.Pow(10, float64(currencyExponent(currency)))
	minor := int64(math.Round(major * factor))
	return &minor, nil
}

// --- result shapers (mirror the hubspot integration's contract) ---

// ObjectResult wraps a single Stripe object result. The object is JSON round-
// tripped to a plain map so downstream nodes can reach ${input.result.<field>}.
func ObjectResult(obj interface{}, summary string) map[string]interface{} {
	m := toMap(obj)
	id, _ := m["id"].(string)
	return map[string]interface{}{
		"tool_result": summary,
		"id":          id,
		"result":      m,
		"success":     true,
		"error":       "",
	}
}

// ListResult wraps a list of Stripe objects.
func ListResult(items []map[string]interface{}, hasMore bool, summary string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": summary,
		"results":     items,
		"count":       len(items),
		"has_more":    hasMore,
		"success":     true,
		"error":       "",
	}
}

// ErrorResult is a graceful failure — success=false, not a node error — so a
// card decline or invalid parameter can be handled in the flow.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// MapError converts a Stripe SDK error into a graceful ErrorResult, surfacing
// the human-readable message from *stripe.Error (card errors, invalid params).
func MapError(err error) map[string]interface{} {
	if se, ok := err.(*stripe.Error); ok && se.Msg != "" {
		return ErrorResult(se.Msg)
	}
	return ErrorResult(err.Error())
}

// ToMap JSON round-trips any Stripe object to a plain map (exported for actions
// that need to shape list items).
func ToMap(v interface{}) map[string]interface{} { return toMap(v) }

func toMap(v interface{}) map[string]interface{} {
	if v == nil {
		return map[string]interface{}{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

type missingInputError struct{ name string }

func (e *missingInputError) Error() string { return e.name + " is required" }

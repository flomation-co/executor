// Package script_common holds helpers shared by the Python and
// JavaScript code action packages. Kept in actions/script/ (not its
// own subdirectory) so the manifest generator's "skip files without
// an Execute function" rule sees it as category-shared infrastructure
// rather than a sibling action.
package script_common

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

// Limits — exposed as exported constants so the action packages can
// describe them in their property-menu help text without re-deriving.
// Tuned per the signed-off design table in the planning conversation:
// default 30s wall-clock, 128 MB memory, 64 KB code, 1 MB output.
const (
	DefaultTimeoutSeconds = 30
	MaxTimeoutSeconds     = 300

	DefaultMemoryMB = 128
	MaxMemoryMB     = 512

	// MaxCodeBytes caps the user script size. 64 KB is generous for
	// hand-written scripts and refuses pasted minified bundles or
	// accidentally-embedded large blobs that would never run cleanly
	// in the sandboxed runtime anyway.
	MaxCodeBytes = 64 * 1024

	// MaxOutputBytes caps the JSON-encoded return value AND the
	// captured logs each. Truncation appends "...truncated" so the
	// caller knows the value was cut.
	MaxOutputBytes    = 1 << 20 // 1 MB
	MaxLogBytes       = 256 * 1024
	MaxOutputBytesCap = 16 << 20 // 16 MB, configurable ceiling
)

// OutputSpec is one row of the user-declared "Outputs" key/value
// array. The Key is the name surfaced to downstream nodes; the
// Value is the declared type used by editor autocomplete and by the
// runtime to coerce/validate the returned value.
//
// Supported declared types: string, number, boolean, object, array.
// Unknown types are accepted and passed through unchanged so the
// user can declare e.g. "datetime" and have the runtime not fight
// them — the worst that happens is the downstream type-coercion
// silently does the right thing.
type OutputSpec struct {
	Name string
	Type string
}

// ParseOutputSpecs reads the "outputs" key/value-array input from
// the action's Connection slice and returns the user's declared
// output catalogue. Empty rows (missing Key) are skipped; duplicate
// Keys keep the LAST declaration (matching how the editor's
// key/value UI presents them — append-only with the last write
// winning).
func ParseOutputSpecs(inputs []*core.Connection, inputName string) []OutputSpec {
	conn := core.FindConnection(inputName, inputs)
	if conn == nil {
		return nil
	}
	pairs := conn.KeyValuePairs()
	seen := make(map[string]int, len(pairs))
	specs := make([]OutputSpec, 0, len(pairs))
	for _, kv := range pairs {
		key := strings.TrimSpace(kv.Key)
		if key == "" {
			continue
		}
		typ := strings.TrimSpace(kv.Value)
		if typ == "" {
			typ = "string"
		}
		if idx, ok := seen[key]; ok {
			specs[idx] = OutputSpec{Name: key, Type: typ}
			continue
		}
		seen[key] = len(specs)
		specs = append(specs, OutputSpec{Name: key, Type: typ})
	}
	return specs
}

// BuildOutputs takes the user's declared output catalogue and the
// raw result object returned by the script, and assembles the final
// outputs map the executor will hand to downstream nodes.
//
// Rules:
//
//   - If the user declared outputs, every declared name is included
//     in the result map (missing values populate as their type's
//     zero value: "" for string, nil for unknown types). This keeps
//     downstream ${input.X} resolution stable when a code path in
//     the script forgets to assign one of the outputs.
//
//   - Anything the script returned that ISN'T in the declared list
//     is silently dropped — declared outputs are the contract.
//
//   - If the user declared NO outputs, the entire result object is
//     emitted under the single "result" key. This preserves the
//     "I just want to script something quickly" path without
//     forcing the user to predeclare names.
//
// The "tool_result" and "logs" outputs are NOT touched here — the
// caller adds them after this returns so they survive whichever
// branch above ran.
func BuildOutputs(specs []OutputSpec, result map[string]interface{}) map[string]interface{} {
	if len(specs) == 0 {
		return map[string]interface{}{
			"result": result,
		}
	}
	out := make(map[string]interface{}, len(specs))
	for _, spec := range specs {
		if v, ok := result[spec.Name]; ok {
			out[spec.Name] = v
			continue
		}
		out[spec.Name] = zeroForType(spec.Type)
	}
	return out
}

// zeroForType returns the "missing value" placeholder appropriate
// for the declared output type. Keeps downstream code from having
// to nil-check the standard types.
func zeroForType(typ string) interface{} {
	switch strings.ToLower(typ) {
	case "string":
		return ""
	case "number", "integer", "float":
		return float64(0)
	case "boolean", "bool":
		return false
	case "array":
		return []interface{}{}
	case "object":
		return map[string]interface{}{}
	default:
		return nil
	}
}

// ─── Script inputs ─────────────────────────────────────────────────
//
// A script action's `inputs` map is assembled from two sources, so authors get
// an ergonomic named-row editor while older flows keep working:
//
//  1. input_vars — a key/value array (name → value). This is the RECOMMENDED
//     path: one row per input, the value a literal or a ${...} reference. Because
//     each row is named, `inputs.<name>` is exactly what you typed — no
//     array-vs-object confusion (the trap of wiring a bare ${array} into a single
//     object field). Each row's (already-substituted) string value is JSON-coerced
//     so arrays/objects/numbers/bools arrive typed.
//  2. inputs_data — the legacy single JSON-object field, kept as a fallback so
//     existing flows are untouched. A non-object value (e.g. a bare array) yields
//     nothing here — that is the very footgun input_vars removes.
//
// input_vars overlays inputs_data (a named row wins over the same key in the
// object), so a flow can migrate incrementally.

// coerceInputValue turns a substituted key/value-row string into a typed value:
// valid JSON (array / object / number / bool / quoted-string) is decoded, and
// anything else is kept as the raw string. An empty string stays "".
func coerceInputValue(s string) interface{} {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	var v interface{}
	if err := json.Unmarshal([]byte(t), &v); err == nil {
		return v
	}
	return s
}

// readInputsDataObject reads the legacy single "inputs_data" object field.
// Accepts a native map (wired directly from an upstream node) or a JSON-object
// string. A non-object value (a bare array, a scalar) yields nil — callers must
// treat that as "no legacy inputs", NOT as an error.
func readInputsDataObject(inputs []*core.Connection) map[string]interface{} {
	conn := core.FindConnection("inputs_data", inputs)
	if conn == nil || conn.Value == nil {
		return nil
	}
	if obj, ok := conn.Value.(map[string]interface{}); ok {
		return obj
	}
	if s, ok := conn.Value.(string); ok {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			return nil
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			return parsed
		}
	}
	return nil
}

// BuildScriptInputs assembles the `inputs` map exposed to a script action from
// the legacy inputs_data object (fallback) overlaid with the named input_vars
// rows (recommended). See the section comment above for the precedence rules.
func BuildScriptInputs(inputs []*core.Connection) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range readInputsDataObject(inputs) {
		out[k] = v
	}
	if conn := core.FindConnection("input_vars", inputs); conn != nil {
		for _, kv := range conn.KeyValuePairs() {
			key := strings.TrimSpace(kv.Key)
			if key == "" {
				continue
			}
			out[key] = coerceInputValue(kv.Value)
		}
	}
	return out
}

// Truncate caps a string at the given byte limit and appends a
// clearly-visible marker so callers know the value was cut. Used
// for both stdout/log capture and the final JSON-encoded result.
func Truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n...truncated"
}

// ExecutionContext returns a child context with the user-configured
// timeout applied. Centralised so both runtimes consistently honour
// the same default / max bounds and the same DeadlineExceeded
// error shape.
func ExecutionContext(parent context.Context, timeoutSeconds int) (context.Context, context.CancelFunc) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = DefaultTimeoutSeconds
	}
	if timeoutSeconds > MaxTimeoutSeconds {
		timeoutSeconds = MaxTimeoutSeconds
	}
	return context.WithTimeout(parent, time.Duration(timeoutSeconds)*time.Second)
}

// ─── Secret-text detector ──────────────────────────────────────────
//
// Mirror of the editor's lib/secretDetection.ts. The editor blocks
// execution when a literal-shaped secret is detected in any input
// value, but the executor enforces independently: a flow saved via
// API (bypassing the editor) or an older flow that pre-dates the
// editor's gate could still arrive here with a hardcoded token.
// Defence-in-depth — refuse to run.

// Unlike the editor's TS detector, the executor scans the WHOLE
// script body — not just one field's value — so anchors are
// dropped. We use word boundaries where possible to keep
// false-positive risk down on substring matches inside larger
// identifiers (`isXOXB` shouldn't trigger), but the unique prefix
// shapes (`xoxb-`, `glpat-`, `AKIA…`) are specific enough that the
// risk is small even without anchoring.
var secretPatterns = []*regexp.Regexp{
	// Stripe / generic sk_/pk_/rk_ tokens
	regexp.MustCompile(`\b(sk|pk|rk)[-_][a-zA-Z0-9]{20,}`),
	// GitHub PATs: ghp/gho/ghu/ghs/ghr
	regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[a-zA-Z0-9]{20,}`),
	// Slack: xoxb/xoxp/xoxs/xoxa + xapp
	regexp.MustCompile(`\bxox[bpsa]-[a-zA-Z0-9-]+`),
	regexp.MustCompile(`\bxapp-[a-zA-Z0-9-]+`),
	// AWS access key ID
	regexp.MustCompile(`\bAKIA[A-Z0-9]{16}`),
	// JWT
	regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]{20,}\.[a-zA-Z0-9_-]+`),
	// GitLab
	regexp.MustCompile(`\bglpat-[a-zA-Z0-9_-]{20,}`),
	// Anthropic
	regexp.MustCompile(`\bsk-ant-[a-zA-Z0-9_-]{20,}`),
	// PEM private key block
	regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY`),
}

// DetectSecretInCode returns a non-empty error string when the
// supplied script body contains a literal-shaped secret. Returns
// "" when clean. Callers should fail the node with the returned
// message — same severity the editor applies to typed inputs.
//
// References like ${secrets.NAME} or ${credentials.NAME} are SAFE
// and explicitly ignored; only literal tokens are flagged. This
// matches the editor's exemption rule so users can paste working
// flows in confidence.
func DetectSecretInCode(code string) string {
	// Strip variable references first so they don't pollute the
	// pattern matchers. We don't need precise positions — only a
	// boolean signal.
	cleaned := stripVariableRefs(code)
	for _, p := range secretPatterns {
		if loc := p.FindStringIndex(cleaned); loc != nil {
			return "Script body contains what looks like a literal secret — store it in environment secrets and reference it as ${secrets.NAME} from your code instead."
		}
	}
	return ""
}

var refStripper = regexp.MustCompile(`\$\{(secrets?|credentials?)\.[^}]+}`)

func stripVariableRefs(code string) string {
	return refStripper.ReplaceAllString(code, "")
}

// FailWithSecret returns the standard error-result map for the
// "literal secret detected" failure. Kept here so both Python and
// JavaScript actions emit identical-shaped errors.
func FailWithSecret(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}, fmt.Errorf("%s", msg)
}

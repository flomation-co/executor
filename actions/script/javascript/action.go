// Package script_javascript embeds a JavaScript runtime (goja —
// pure-Go, no CGo) inside the executor so users can express
// transformation logic in ES2020+ without the host system needing
// Node.js installed. See the design notes in the planning thread
// for why goja over v8go / quickjs (no CGo, single binary,
// predictable wallclock).
package script_javascript

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"

	core "flomation.app/automate/executor"
	script_common "flomation.app/automate/executor/actions/script"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Run JavaScript"
	Description  = "Execute a sandboxed JavaScript script (ES2020+, no network, no file system)"
	Website      = "https://www.flomation.co"
	Icon         = "code"
	Date         = "15/06/2026"
	Type         = core.ActionTypeAction
)

// versionLabel is what we render in the dropdown and round-trip in
// saved flows. Even with one option today, the dropdown is shipped
// up-front so future runtimes (V8 via v8go, QuickJS) add cleanly
// without breaking the manifest schema.
const versionLabel = "ES2020 (goja)"

var Inputs = [...]core.Connection{
	{
		Name:        "code",
		Type:        core.ConnectionTypeCode,
		Label:       "JavaScript Code",
		Placeholder: "// Inputs are available on the global `inputs` object.\n// Return an object whose keys match your declared outputs, or assign to outputs.X.\nreturn { total: inputs.a + inputs.b };",
		Required:    true,
	},
	{
		// String literals are required here — the manifest
		// generator's AST inspector resolves only literals, not
		// const references. Using `versionLabel` here would leave
		// the option's name/value empty in the manifest and the
		// editor would render the field as an empty text input.
		// See CLAUDE.md's note about literal composite literals.
		Name:  "version",
		Type:  core.ConnectionTypeString,
		Label: "JavaScript Version",
		Options: []core.ConnectionOption{
			{Name: "ES2020 (goja)", Value: "ES2020 (goja)"},
		},
	},
	{
		Name:  "outputs",
		Type:  core.ConnectionTypeKeyValueArray,
		Label: "Outputs (name → type)",
		Options: []core.ConnectionOption{
			{Name: "String", Value: "string"},
			{Name: "Number", Value: "number"},
			{Name: "Boolean", Value: "boolean"},
			{Name: "Array", Value: "array"},
			{Name: "Object", Value: "object"},
		},
		Placeholder: "e.g. total → number, status → string. Leave empty to emit the whole return value under \"result\".",
	},
	{
		Name:        "input_vars",
		Type:        core.ConnectionTypeKeyValueArray,
		Label:       "Inputs",
		Placeholder: "Add each input by name; the value can be a ${...} reference. Read it in the script as inputs.NAME.",
	},
	{
		Name:        "inputs_data",
		Type:        core.ConnectionTypeObject,
		Label:       "Inputs (JSON object — advanced)",
		Placeholder: "Optional. A whole JSON object merged under the named Inputs above. Prefer the named rows.",
	},
	{
		Name:        "timeout_seconds",
		Type:        core.ConnectionTypeInteger,
		Label:       "Timeout (seconds)",
		Placeholder: "30",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "logs", Type: core.ConnectionTypeString, Label: "Console output"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "duration_ms", Type: core.ConnectionTypeInteger, Label: "Duration (ms)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// 1. Code text — required, with the secret-detection guard
	// applied so a flow saved via API can't slip a literal token
	// past the editor's enforcement.
	codeConn := core.FindConnection("code", inputs)
	if codeConn == nil || codeConn.String() == nil || strings.TrimSpace(*codeConn.String()) == "" {
		return errResult("code is required"), errors.New("code is required")
	}
	code := *codeConn.String()
	if len(code) > script_common.MaxCodeBytes {
		msg := fmt.Sprintf("script body exceeds %d bytes — split it into a sub-flow", script_common.MaxCodeBytes)
		return errResult(msg), errors.New(msg)
	}
	if leak := script_common.DetectSecretInCode(code); leak != "" {
		return script_common.FailWithSecret(leak)
	}

	// 2. Output catalogue — empty means "emit whole return under
	// `result`"; non-empty constrains the output map to the user's
	// declared keys.
	specs := script_common.ParseOutputSpecs(inputs, "outputs")

	// 3. User-supplied inputs payload — the named input_vars rows
	// (recommended) overlaid on the legacy inputs_data object. See
	// script_common.BuildScriptInputs for the precedence rules.
	scriptInputs := script_common.BuildScriptInputs(inputs)

	// 4. Timeout
	timeoutSecs := script_common.DefaultTimeoutSeconds
	if tc := core.FindConnection("timeout_seconds", inputs); tc != nil && tc.Number() != nil {
		timeoutSecs = int(*tc.Number())
	}
	ctx, cancel := script_common.ExecutionContext(flow.GoContext(), timeoutSecs)
	defer cancel()

	start := time.Now()
	result, logs, err := runScript(ctx, code, scriptInputs)
	durationMs := time.Since(start).Milliseconds()

	// 5. Map runtime failure modes to a single, consistent error
	// shape so the AI tool loop sees a clean failure.
	if err != nil {
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			msg = fmt.Sprintf("JavaScript script exceeded timeout of %d seconds", timeoutSecs)
		}
		out := map[string]interface{}{
			"tool_result": msg,
			"logs":        script_common.Truncate(logs, script_common.MaxLogBytes),
			"success":     false,
			"error":       msg,
			"duration_ms": durationMs,
		}
		return out, errors.New(msg)
	}

	// 6. Build the outputs map per the user's declared catalogue.
	// BuildOutputs handles the "no declared outputs" case by
	// wrapping under "result".
	out := script_common.BuildOutputs(specs, result)

	// 7. Standard meta-outputs alongside whatever the script
	// declared. tool_result is a short human-readable summary;
	// logs are surfaced for debugging.
	out["tool_result"] = summariseSuccess(result, specs)
	out["logs"] = script_common.Truncate(logs, script_common.MaxLogBytes)
	out["success"] = true
	out["duration_ms"] = durationMs
	return out, nil
}

// runScript spins up a goja VM, wraps the user code in an IIFE so
// `return` works at the top level, installs the inputs/outputs/
// console bindings, and executes. Cancellation respects the
// context — when it fires we call vm.Interrupt() to bail the VM out
// of any in-progress instruction (including infinite loops).
func runScript(ctx context.Context, code string, scriptInputs map[string]interface{}) (map[string]interface{}, string, error) {
	vm := goja.New()

	// Bind inputs as a frozen-ish object. goja accepts native Go
	// values via ToValue and converts JSON-shaped trees into
	// proper JS objects (so `inputs.user.name` works).
	if err := vm.Set("inputs", scriptInputs); err != nil {
		return nil, "", fmt.Errorf("bind inputs: %w", err)
	}

	// Outputs object — supports the mutation style ("outputs.x = 1")
	// alongside the return-value style. After the script runs we
	// merge both, with the return value winning when both are set.
	outputsObj := map[string]interface{}{}
	if err := vm.Set("outputs", outputsObj); err != nil {
		return nil, "", fmt.Errorf("bind outputs: %w", err)
	}

	// console.log / .info / .warn / .error — every level is captured
	// to the same logs buffer. The native objects don't exist in
	// goja unless we install them; doing so explicitly also lets us
	// enforce the log size cap.
	var logBuf strings.Builder
	logFn := func(prefix string) func(args ...interface{}) {
		return func(args ...interface{}) {
			if logBuf.Len() >= script_common.MaxLogBytes {
				return
			}
			parts := make([]string, len(args))
			for i, a := range args {
				parts[i] = stringifyForLog(a)
			}
			if prefix != "" {
				logBuf.WriteString(prefix)
				logBuf.WriteByte(' ')
			}
			logBuf.WriteString(strings.Join(parts, " "))
			logBuf.WriteByte('\n')
		}
	}
	consoleObj := vm.NewObject()
	_ = consoleObj.Set("log", logFn(""))
	_ = consoleObj.Set("info", logFn(""))
	_ = consoleObj.Set("warn", logFn("[warn]"))
	_ = consoleObj.Set("error", logFn("[error]"))
	_ = vm.Set("console", consoleObj)

	// Cancellation: goroutine watches the context, fires Interrupt
	// when it cancels. The Interrupt is observed at the next VM
	// instruction boundary — bounded by goja's interruption check,
	// which is per loop iteration / function call.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt(ctx.Err())
		case <-done:
		}
	}()

	// Wrap the user code in an IIFE so `return` works at the top
	// level and bindings inside the script don't pollute global
	// scope across runs.
	wrapped := "(function(){\n" + code + "\n})()"

	val, err := vm.RunString(wrapped)
	if err != nil {
		// Interrupt errors come through as *goja.InterruptedError;
		// surface the underlying context error to the caller so the
		// timeout branch above triggers cleanly.
		var ierr *goja.InterruptedError
		if errors.As(err, &ierr) {
			if cerr, ok := ierr.Value().(error); ok {
				return nil, logBuf.String(), cerr
			}
		}
		return nil, logBuf.String(), err
	}

	// Merge return value (wins) over outputs object (mutation).
	merged := map[string]interface{}{}
	for k, v := range outputsObj {
		merged[k] = v
	}
	if val != nil && !goja.IsUndefined(val) && !goja.IsNull(val) {
		// Export converts goja Values into plain Go values that
		// the rest of the executor can serialise as JSON.
		exported := val.Export()
		if asMap, ok := exported.(map[string]interface{}); ok {
			for k, v := range asMap {
				merged[k] = v
			}
		} else if len(outputsObj) == 0 {
			// User returned a non-object (number, string, array) and
			// didn't use outputs.X mutation. Honour it as the single
			// "result" value — BuildOutputs will wrap it correctly.
			merged["result"] = exported
		}
	}
	return merged, logBuf.String(), nil
}

func stringifyForLog(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return "undefined"
	default:
		if b, err := json.Marshal(t); err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", v)
	}
}

func summariseSuccess(result map[string]interface{}, specs []script_common.OutputSpec) string {
	if len(specs) == 0 {
		return fmt.Sprintf("script ran successfully (%d top-level keys returned)", len(result))
	}
	return fmt.Sprintf("script ran successfully (%d declared outputs)", len(specs))
}

func errResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"logs":        "",
		"success":     false,
		"error":       msg,
		"duration_ms": int64(0),
	}
}

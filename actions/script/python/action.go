package script_python

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"

	core "flomation.app/automate/executor"
	script_common "flomation.app/automate/executor/actions/script"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Run Python"
	Description  = "Execute a sandboxed Python 3.12 script (CPython-WASI, no network, /work only)"
	Website      = "https://www.flomation.co"
	Icon         = "code"
	Date         = "15/06/2026"
	Type         = core.ActionTypeAction
)

const versionLabel = "Python 3.12 (WASI)"

var Inputs = [...]core.Connection{
	{
		Name:        "code",
		Type:        core.ConnectionTypeCode,
		Label:       "Python Code",
		Placeholder: "# Inputs are in the `inputs` dict; assign to `outputs`\n# or return a dict — either form populates the outputs.\noutputs['total'] = inputs['a'] + inputs['b']\nreturn {'status': 'ok'}",
		Required:    true,
	},
	{
		// Manifest generator requires string literals here — see
		// the matching note in script/javascript/action.go for why
		// a const reference fails silently.
		Name:  "version",
		Type:  core.ConnectionTypeString,
		Label: "Python Version",
		Options: []core.ConnectionOption{
			{Name: "Python 3.12 (WASI)", Value: "Python 3.12 (WASI)"},
		},
	},
	{
		Name:        "outputs",
		Type:        core.ConnectionTypeKeyValueArray,
		Label:       "Outputs (name → type)",
		Placeholder: "e.g. total → number, status → string. Leave empty to emit the whole outputs dict under \"result\".",
	},
	{
		Name:        "inputs_data",
		Type:        core.ConnectionTypeObject,
		Label:       "Inputs",
		Placeholder: "Available inside the script as `inputs['X']`. Wire variables in from upstream nodes.",
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
	{Name: "logs", Type: core.ConnectionTypeString, Label: "stdout"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "duration_ms", Type: core.ConnectionTypeInteger, Label: "Duration (ms)"},
}

// Shared wazero runtime + compiled CPython module — built once on
// first use and reused across executions. Compilation is the
// expensive step (~2-3s for the full CPython wasm); per-execution
// cost is then just module instantiation (~150ms).
var (
	runtimeOnce sync.Once
	runtimeRT   wazero.Runtime
	runtimeMod  wazero.CompiledModule
	runtimeErr  error
)

func ensureRuntime(ctx context.Context) (wazero.Runtime, wazero.CompiledModule, error) {
	runtimeOnce.Do(func() {
		wasm, err := loadCPythonWasm()
		if err != nil {
			runtimeErr = err
			return
		}
		cfg := wazero.NewRuntimeConfig().
			// Memory cap applies to the compiled module instances.
			// 128 MB default keeps user code from exhausting the
			// host; can be raised per-execution if needed.
			WithMemoryLimitPages(uint32(script_common.DefaultMemoryMB * 16))
		rt := wazero.NewRuntimeWithConfig(ctx, cfg)
		if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
			runtimeErr = fmt.Errorf("instantiate WASI: %w", err)
			return
		}
		mod, err := rt.CompileModule(ctx, wasm)
		if err != nil {
			runtimeErr = fmt.Errorf("compile cpython module: %w", err)
			return
		}
		runtimeRT = rt
		runtimeMod = mod
	})
	return runtimeRT, runtimeMod, runtimeErr
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// 1. Code text + secret-detection guard
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

	specs := script_common.ParseOutputSpecs(inputs, "outputs")
	scriptInputs := readScriptInputs(inputs)

	timeoutSecs := script_common.DefaultTimeoutSeconds
	if tc := core.FindConnection("timeout_seconds", inputs); tc != nil && tc.Number() != nil {
		timeoutSecs = int(*tc.Number())
	}
	ctx, cancel := script_common.ExecutionContext(flow.GoContext(), timeoutSecs)
	defer cancel()

	start := time.Now()
	result, logs, err := runScript(ctx, code, scriptInputs)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			msg = fmt.Sprintf("Python script exceeded timeout of %d seconds", timeoutSecs)
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

	out := script_common.BuildOutputs(specs, result)
	out["tool_result"] = summariseSuccess(result, specs)
	out["logs"] = script_common.Truncate(logs, script_common.MaxLogBytes)
	out["success"] = true
	out["duration_ms"] = durationMs
	return out, nil
}

// runScript spins up a sandboxed Python interpreter and runs the
// user code. The contract on the Python side:
//
//   - A bootstrap preamble injects two globals: `inputs` (dict) and
//     `outputs` (dict). The user code runs after the preamble.
//
//   - After the user code finishes, a postamble JSON-encodes the
//     `outputs` dict and prints a sentinel line we can scan for in
//     stdout. Anything else printed by the user is collected as
//     logs.
//
// We mount a per-execution temp directory at /work so user code
// can read/write files; no other host paths are exposed and no
// network access is granted (WASI capabilities-based sandbox).
func runScript(ctx context.Context, code string, scriptInputs map[string]interface{}) (map[string]interface{}, string, error) {
	rt, mod, err := ensureRuntime(ctx)
	if err != nil {
		return nil, "", err
	}

	workDir, err := os.MkdirTemp("", "flomation-py-*")
	if err != nil {
		return nil, "", fmt.Errorf("create workdir: %w", err)
	}
	defer os.RemoveAll(workDir)

	inputsJSON, err := json.Marshal(scriptInputs)
	if err != nil {
		return nil, "", fmt.Errorf("marshal inputs: %w", err)
	}

	// Write the user script and the inputs payload to the workdir,
	// where the bootstrap preamble can read them in. Passing them
	// as files keeps the wazero argv / stdin small (some WASI
	// stdlib layers truncate long argv).
	if err := os.WriteFile(filepath.Join(workDir, "_inputs.json"), inputsJSON, 0600); err != nil {
		return nil, "", fmt.Errorf("write inputs file: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "_user.py"), []byte(code), 0600); err != nil {
		return nil, "", fmt.Errorf("write user script: %w", err)
	}

	// Bootstrap script: wraps the user code in a function so that
	// top-level `return` works (matching the JavaScript node's IIFE
	// behaviour), then captures both the function's return value
	// AND any mutations the user made to the `outputs` dict.
	//
	// AST manipulation is used instead of textwrap.indent + string
	// prepend because it preserves the user's original line numbers
	// — tracebacks fired from inside the wrapped function still
	// point at the line the user actually wrote, not at an offset
	// shifted by however many wrapper lines we prepended.
	//
	// Semantics:
	//   - `inputs` (dict) and `outputs` (dict) are passed in as
	//     parameters so the user can reference them by their
	//     natural names without any "global" gymnastics.
	//   - Mutating outputs (outputs['x'] = 1) persists because
	//     dict mutation is by reference.
	//   - Returning a dict merges on top of the mutated outputs,
	//     so explicit `return` wins over implicit mutation when
	//     both touch the same key.
	//   - Returning a non-dict value lands under outputs['result']
	//     — mirrors the JS node's wrap-non-objects rule.
	bootstrap := `import ast, json, sys
with open('/work/_inputs.json') as f:
    inputs = json.load(f)
outputs = {}
with open('/work/_user.py') as f:
    src = f.read()
user_tree = ast.parse(src, filename='<flomation>')
wrapper = ast.parse('def __flomation_main(inputs, outputs):\n    pass\n')
if user_tree.body:
    wrapper.body[0].body = user_tree.body
ast.fix_missing_locations(wrapper)
ns = {'inputs': inputs, 'outputs': outputs}
exec(compile(wrapper, '<flomation>', 'exec'), ns)
result = ns['__flomation_main'](inputs, outputs)
if isinstance(result, dict):
    outputs.update(result)
elif result is not None:
    outputs['result'] = result
sys.stdout.write('\n__FLOMATION_OUTPUTS__\n' + json.dumps(outputs, default=str) + '\n')
`
	if err := os.WriteFile(filepath.Join(workDir, "_bootstrap.py"), []byte(bootstrap), 0600); err != nil {
		return nil, "", fmt.Errorf("write bootstrap: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	modCfg := wazero.NewModuleConfig().
		WithStdin(bytes.NewReader(nil)).
		WithStdout(&stdoutBuf).
		WithStderr(&stderrBuf).
		// FS scope: workdir mounted at /work, nothing else exposed.
		// Critically NO sockets capability is granted, so socket() /
		// connect() return ENOSYS from inside the script.
		WithFSConfig(wazero.NewFSConfig().WithDirMount(workDir, "/work")).
		// argv: python invoked with the bootstrap script path
		WithArgs("python", "/work/_bootstrap.py").
		// Clean env — host environment is NOT inherited.
		WithEnv("PYTHONDONTWRITEBYTECODE", "1").
		WithEnv("PYTHONUNBUFFERED", "1").
		// Random source — needed for hash randomisation in 3.12.
		WithRandSource(nil).
		// startfunctions: needs to be the empty name for WASI command modules
		WithStartFunctions("_start")

	_, runErr := rt.InstantiateModule(ctx, mod, modCfg)
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// Recover outputs from the stdout sentinel line. Anything
	// before the sentinel is treated as user logs.
	logs, outputsJSON := splitAtSentinel(stdout)
	if stderr != "" {
		if logs != "" {
			logs += "\n"
		}
		logs += "[stderr]\n" + stderr
	}

	if runErr != nil {
		// ExitError(0) is a normal "python finished" signal — wazero
		// surfaces a successful run that way for command modules.
		// Treat as success and fall through to parse outputs.
		var exitErr *sys.ExitError
		if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 0 {
			return nil, logs, scriptError(ctx, runErr, stderr)
		}
	}
	if outputsJSON == "" {
		return nil, logs, fmt.Errorf("python script did not produce an outputs payload")
	}
	var outputs map[string]interface{}
	if err := json.Unmarshal([]byte(outputsJSON), &outputs); err != nil {
		return nil, logs, fmt.Errorf("parse outputs JSON: %w", err)
	}
	return outputs, logs, nil
}

const outputsSentinel = "__FLOMATION_OUTPUTS__\n"

// splitAtSentinel separates user-printed stdout (logs) from our
// bootstrap-printed outputs JSON. The sentinel is a string the
// user is extremely unlikely to print themselves, and even if they
// do the last occurrence wins — outputs are always the final
// thing printed.
func splitAtSentinel(stdout string) (logs, outputsJSON string) {
	idx := strings.LastIndex(stdout, outputsSentinel)
	if idx == -1 {
		return stdout, ""
	}
	logs = stdout[:idx]
	rest := stdout[idx+len(outputsSentinel):]
	// Trim trailing newline added by the bootstrap.
	outputsJSON = strings.TrimRight(rest, "\n")
	return strings.TrimRight(logs, "\n"), outputsJSON
}

func scriptError(ctx context.Context, runErr error, stderr string) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if stderr != "" {
		return fmt.Errorf("python script failed: %s", strings.TrimSpace(stderr))
	}
	return fmt.Errorf("python script failed: %w", runErr)
}

func readScriptInputs(inputs []*core.Connection) map[string]interface{} {
	conn := core.FindConnection("inputs_data", inputs)
	if conn == nil || conn.Value == nil {
		return map[string]interface{}{}
	}
	if obj, ok := conn.Value.(map[string]interface{}); ok {
		return obj
	}
	if s, ok := conn.Value.(string); ok {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			return map[string]interface{}{}
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			return parsed
		}
	}
	return map[string]interface{}{}
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

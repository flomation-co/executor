package opentofu

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/opentofu/tofu"
)

// maxOutputBytes caps the stdout/stderr surfaced from a tofu invocation so a
// large plan/apply log can't bloat the flow's node results.
const maxOutputBytes = 1 << 20

// SharedInputs is the canonical set of inputs that every OpenTofu action exposes
// identically: the working directory, variable/backend wiring, the backend-auth
// provider block, binary selection, and the local-state guard.
//
// It is the single source of truth, but each action ALSO declares these entries
// inline in its own `var Inputs = [...]core.Connection{…}`. That duplication is
// forced by the manifest generator (cmd/manifest), which only extracts an inline
// composite literal and cannot resolve a shared package var — so an action that
// referenced this var directly would have its fields vanish from the editor UI.
// TestSharedInputsDoNotDrift asserts the inline copies stay byte-identical to
// this list, so drift is caught in CI. Removing the duplication requires teaching
// the generator to compose a shared var (tracked in issue #1).
var SharedInputs = []core.Connection{
	{
		Name:        "working_directory",
		Type:        core.ConnectionTypeString,
		Label:       "Working Directory",
		Placeholder: "/path/to/terraform/config",
		Required:    true,
	},
	{
		Name:        "variables",
		Type:        core.ConnectionTypeKeyValueArray,
		Label:       "Variables",
		Placeholder: "Exported as TF_VAR_<name>",
	},
	{
		Name:        "backend_config",
		Type:        core.ConnectionTypeKeyValueArray,
		Label:       "Backend Config",
		Placeholder: "Passed to `tofu init -backend-config` (use a remote backend)",
	},
	{
		Name:        "credentials",
		Type:        core.ConnectionTypeKeyValueArray,
		Label:       "Environment Credentials",
		Placeholder: "Extra provider/backend env vars, e.g. AWS_ACCESS_KEY_ID → ${secrets.aws_key}",
	},
	{
		Name:  "backend_auth",
		Type:  core.ConnectionTypeString,
		Label: "Backend Authentication",
		Options: []core.ConnectionOption{
			{Name: "None / use credentials above", Value: ""},
			{Name: "AWS (S3 / DynamoDB)", Value: "aws"},
			{Name: "Azure (azurerm)", Value: "azure"},
			{Name: "Google Cloud (gcs)", Value: "gcp"},
			{Name: "GitLab (http)", Value: "gitlab"},
		},
	},
	{Name: "aws_access_key_id", Type: core.ConnectionTypeSecret, Label: "AWS Access Key ID", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"aws"}}},
	{Name: "aws_secret_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Access Key", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"aws"}}},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "AWS Session Token (optional)", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"aws"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "AWS Region", Placeholder: "eu-west-2", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"aws"}}},
	{Name: "arm_client_id", Type: core.ConnectionTypeSecret, Label: "Azure Client ID", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"azure"}}},
	{Name: "arm_client_secret", Type: core.ConnectionTypeSecret, Label: "Azure Client Secret", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"azure"}}},
	{Name: "arm_tenant_id", Type: core.ConnectionTypeSecret, Label: "Azure Tenant ID", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"azure"}}},
	{Name: "arm_subscription_id", Type: core.ConnectionTypeSecret, Label: "Azure Subscription ID", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"azure"}}},
	{Name: "arm_access_key", Type: core.ConnectionTypeSecret, Label: "Azure Storage Access Key (optional)", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"azure"}}},
	{Name: "gcp_credentials_json", Type: core.ConnectionTypeSecret, Label: "GCP Service Account JSON", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"gcp"}}},
	{Name: "gitlab_username", Type: core.ConnectionTypeString, Label: "GitLab Username", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"gitlab"}}},
	{Name: "gitlab_token", Type: core.ConnectionTypeSecret, Label: "GitLab Token", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"gitlab"}}},
	{Name: "gitlab_address", Type: core.ConnectionTypeString, Label: "GitLab State Address", Placeholder: "https://gitlab.com/api/v4/projects/<id>/terraform/state/<name>", Visible: &core.VisibleWhen{Field: "backend_auth", Values: []string{"gitlab"}}},
	{
		Name:        "tofu_version",
		Type:        core.ConnectionTypeString,
		Label:       "OpenTofu Version",
		Placeholder: "1.9.1 (pinned default)",
	},
	{
		Name:        "binary_path",
		Type:        core.ConnectionTypeString,
		Label:       "Binary Path (optional)",
		Placeholder: "Use a host-installed tofu instead of downloading",
	},
	{
		Name:    "allow_local_state",
		Type:    core.ConnectionTypeBoolean,
		Label:   "Allow Local State (unsafe)",
		Options: []core.ConnectionOption{{Name: "No", Value: "false"}, {Name: "Yes", Value: "true"}},
	},
}

// OptStr returns the trimmed string value of an input, or "" when unset.
func OptStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return strings.TrimSpace(*c.String())
}

// OptBool returns a boolean input's value, defaulting to def when unset.
//
// Boolean inputs arrive as the option string "true"/"false" (the editor renders
// them via Options rather than a native bool), so we compare the string rather
// than using Connection.Boolean. Anything other than an explicit "true"/"false"
// falls back to def — this is how a default-on toggle like require_approval is
// expressed without a tri-state.
func OptBool(name string, inputs []*core.Connection, def bool) bool {
	switch OptStr(name, inputs) {
	case "true":
		return true
	case "false":
		return false
	default:
		return def
	}
}

// KVMap parses a key_value_array input into a map, dropping blank keys.
func KVMap(name string, inputs []*core.Connection) map[string]string {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	m := map[string]string{}
	for _, p := range c.KeyValuePairs() {
		if p.Key != "" {
			m[p.Key] = p.Value
		}
	}
	return m
}

// Truncate bounds a captured output stream and trims surrounding whitespace.
func Truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxOutputBytes {
		return s[:maxOutputBytes] + "\n... (output truncated)"
	}
	return s
}

// Timeout parses timeout_seconds, applying defaultSec when unset. It returns an
// error — rather than silently clamping — when the value is non-numeric, not
// positive, or exceeds maxSec, so the user gets immediate, actionable feedback
// instead of a long apply that dies at an unexpected deadline mid-run.
func Timeout(raw string, defaultSec, maxSec int) (time.Duration, error) {
	secs := defaultSec
	if raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, fmt.Errorf("timeout_seconds %q is not a whole number of seconds", raw)
		}
		if n <= 0 {
			return 0, fmt.Errorf("timeout_seconds must be a positive number of seconds")
		}
		if n > maxSec {
			return 0, fmt.Errorf("timeout_seconds %d exceeds the maximum of %d (%s)", n, maxSec, (time.Duration(maxSec) * time.Second).String())
		}
		secs = n
	}
	return time.Duration(secs) * time.Second, nil
}

// WorkDir resolves and validates the working_directory input. Returning the
// error keeps the empty/unresolved/missing-directory checks identical and in the
// same order across every action.
func WorkDir(inputs []*core.Connection) (string, error) {
	wd := OptStr("working_directory", inputs)
	if wd == "" || strings.HasPrefix(wd, "${") {
		return "", fmt.Errorf("working_directory is required")
	}
	if fi, err := os.Stat(wd); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("working_directory %q is not an accessible directory", wd)
	}
	return wd, nil
}

// CheckBackend enforces the remote-backend guard unless the caller has opted
// into local state via allow_local_state.
func CheckBackend(workDir string, inputs []*core.Connection) error {
	if OptBool("allow_local_state", inputs, false) {
		return nil
	}
	return tofu.RequireRemoteBackend(workDir)
}

// Config assembles the tofu.RunConfig shared by every action from the common
// inputs (working directory, binary selection, variables, and backend auth).
func Config(workDir string, inputs []*core.Connection) tofu.RunConfig {
	return tofu.RunConfig{
		WorkDir:       workDir,
		Version:       OptStr("tofu_version", inputs),
		BinaryPath:    OptStr("binary_path", inputs),
		TFVars:        KVMap("variables", inputs),
		ExtraEnv:      backendEnv(inputs),
		BackendConfig: backendConfig(inputs),
	}
}

// backendEnv merges the free-form credentials input with the typed,
// provider-specific backend auth fields (the latter take precedence).
func backendEnv(inputs []*core.Connection) map[string]string {
	env := KVMap("credentials", inputs)
	if env == nil {
		env = map[string]string{}
	}
	auth := tofu.BackendAuthEnv(OptStr("backend_auth", inputs), func(n string) string { return OptStr(n, inputs) })
	for k, v := range auth {
		env[k] = v
	}
	return env
}

// backendConfig merges the provider-derived -backend-config entries (e.g. the
// GitLab http address/lock plumbing) with the user's explicit backend_config
// input. Explicit entries win so a user can always override the derived values.
func backendConfig(inputs []*core.Connection) map[string]string {
	cfg := tofu.BackendConfigFor(OptStr("backend_auth", inputs), func(n string) string { return OptStr(n, inputs) })
	if cfg == nil {
		cfg = map[string]string{}
	}
	for k, v := range KVMap("backend_config", inputs) {
		cfg[k] = v
	}
	return cfg
}

// BaseResult builds the output keys common to every OpenTofu action. Callers add
// their action-specific keys (status, change counts, outputs, …) to the map.
func BaseResult(toolResult, stdout, stderr string, exitCode int, success bool) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": toolResult,
		"stdout":      Truncate(stdout),
		"stderr":      Truncate(stderr),
		"exit_code":   int64(exitCode),
		"success":     success,
	}
}

// ErrResult builds a failure result for an input/setup error where no tofu
// process ran. extra carries the action-specific output keys (with their
// failure-state zero values) so every action keeps a stable output schema.
func ErrResult(msg string, extra map[string]interface{}) map[string]interface{} {
	r := BaseResult("Error: "+msg, "", msg, -1, false)
	for k, v := range extra {
		r[k] = v
	}
	return r
}

// FailResult builds a failure result from a tofu command that ran but exited
// non-zero (or whose preceding init/plan failed). extra is as for ErrResult.
func FailResult(msg string, res *tofu.RunResult, extra map[string]interface{}) map[string]interface{} {
	r := BaseResult(fmt.Sprintf("%s (exit %d)", msg, res.ExitCode), res.Stdout, res.Stderr, res.ExitCode, false)
	for k, v := range extra {
		r[k] = v
	}
	return r
}

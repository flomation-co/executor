// Package opentofu_destroy tears down the resources managed by an OpenTofu
// configuration. Like apply it defaults to pausing for human approval, showing
// a `plan -destroy` summary before suspending the flow.
//
// State note: destroy re-initialises against the configured (remote) backend on
// resume; use a remote backend with locking — local state is not safe across a
// suspend/resume that may move runners.
package opentofu_destroy

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/opentofu/tofu"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "OpenTofu Destroy"
	Description  = "Destroy the resources managed by an OpenTofu (Terraform-compatible) configuration, optionally pausing for human approval first."
	Website      = "https://www.flomation.co"
	Icon         = "box+trash"
	Date         = "18/06/2026"
	Type         = core.ActionTypeAction

	defaultTimeout = 1800 // seconds
	maxTimeout     = 3600 // seconds
	maxOutputBytes = 1 << 20

	approvalReason = "opentofu_destroy_approval"
)

var Inputs = [...]core.Connection{
	{
		Name:        "working_directory",
		Type:        core.ConnectionTypeString,
		Label:       "Working Directory",
		Placeholder: "/path/to/terraform/config",
		Required:    true,
	},
	{
		Name:    "require_approval",
		Type:    core.ConnectionTypeBoolean,
		Label:   "Require Approval",
		Options: []core.ConnectionOption{{Name: "Yes", Value: "true"}, {Name: "No", Value: "false"}},
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
		Name:    "allow_local_state",
		Type:    core.ConnectionTypeBoolean,
		Label:   "Allow Local State (unsafe)",
		Options: []core.ConnectionOption{{Name: "No", Value: "false"}, {Name: "Yes", Value: "true"}},
	},
	{
		Name:        "binary_path",
		Type:        core.ConnectionTypeString,
		Label:       "Binary Path (optional)",
		Placeholder: "Use a host-installed tofu instead of downloading",
	},
	{
		Name:        "timeout_seconds",
		Type:        core.ConnectionTypeInteger,
		Label:       "Timeout (seconds)",
		Placeholder: "1800",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "stdout", Type: core.ConnectionTypeText, Label: "Destroy Output"},
	{Name: "stderr", Type: core.ConnectionTypeText, Label: "Standard Error"},
	{Name: "destroy", Type: core.ConnectionTypeInteger, Label: "Resources to Destroy"},
	{Name: "exit_code", Type: core.ConnectionTypeInteger, Label: "Exit Code"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	workDir := optStr("working_directory", inputs)
	if workDir == "" || strings.HasPrefix(workDir, "${") {
		return errResult("working_directory is required"), nil
	}
	if fi, err := os.Stat(workDir); err != nil || !fi.IsDir() {
		return errResult(fmt.Sprintf("working_directory %q is not an accessible directory", workDir)), nil
	}

	if optStr("allow_local_state", inputs) != "true" {
		if err := tofu.RequireRemoteBackend(workDir); err != nil {
			return errResult(err.Error()), nil
		}
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), timeout(inputs))
	defer cancel()

	cfg := tofu.RunConfig{
		WorkDir:       workDir,
		Version:       optStr("tofu_version", inputs),
		BinaryPath:    optStr("binary_path", inputs),
		TFVars:        kvMap("variables", inputs),
		ExtraEnv:      backendEnv(inputs),
		BackendConfig: backendConfig(inputs),
	}

	requireApproval := optStr("require_approval", inputs) != "false" // default on

	if requireApproval && !flow.IsResumedNode(node.ID) {
		return planAndSuspend(ctx, flow, node, cfg)
	}

	return destroy(ctx, cfg)
}

func planAndSuspend(ctx context.Context, flow *core.Flow, node *core.Node, cfg tofu.RunConfig) (map[string]interface{}, error) {
	bin, env, initRes, err := tofu.Prepare(ctx, cfg)
	if err != nil {
		return errResult(fmt.Sprintf("tofu init failed to start: %v", err)), nil
	}
	if initRes.ExitCode != 0 {
		return failResult("tofu init failed", initRes), nil
	}

	planRes, err := tofu.Run(ctx, bin, cfg.WorkDir, env, "plan", "-destroy", "-input=false", "-no-color", "-json")
	if err != nil {
		return errResult(fmt.Sprintf("tofu plan -destroy failed to run: %v", err)), nil
	}
	if planRes.ExitCode != 0 {
		return failResult("tofu plan -destroy failed", planRes), nil
	}

	summary, _ := tofu.ParsePlanSummary(planRes.Stdout)

	flow.Suspend(&core.SuspendInfo{
		NodeID: node.ID,
		Reason: approvalReason,
	})

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Awaiting approval to destroy %d resource(s)", summary.Destroy),
		"status":      "pending_approval",
		"stdout":      truncate(planRes.Stdout),
		"stderr":      truncate(planRes.Stderr),
		"destroy":     int64(summary.Destroy),
		"exit_code":   int64(0),
		"success":     true,
	}, core.ErrSuspended
}

func destroy(ctx context.Context, cfg tofu.RunConfig) (map[string]interface{}, error) {
	bin, env, initRes, err := tofu.Prepare(ctx, cfg)
	if err != nil {
		return errResult(fmt.Sprintf("tofu init failed to start: %v", err)), nil
	}
	if initRes.ExitCode != 0 {
		return failResult("tofu init failed", initRes), nil
	}

	res, err := tofu.Run(ctx, bin, cfg.WorkDir, env, "destroy", "-input=false", "-no-color", "-auto-approve")
	if err != nil {
		return errResult(fmt.Sprintf("tofu destroy failed to run: %v", err)), nil
	}

	success := res.ExitCode == 0
	toolResult := fmt.Sprintf("Destroy failed (exit %d)", res.ExitCode)
	status := "failed"
	if success {
		toolResult = "Destroy complete"
		status = "destroyed"
	}

	return map[string]interface{}{
		"tool_result": toolResult,
		"status":      status,
		"stdout":      truncate(res.Stdout),
		"stderr":      truncate(res.Stderr),
		"destroy":     int64(0),
		"exit_code":   int64(res.ExitCode),
		"success":     success,
	}, nil
}

// --- local helpers ----------------------------------------------------------

func timeout(inputs []*core.Connection) time.Duration {
	secs := defaultTimeout
	if v := optStr("timeout_seconds", inputs); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			secs = n
		}
	}
	if secs > maxTimeout {
		secs = maxTimeout
	}
	return time.Duration(secs) * time.Second
}

func optStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return strings.TrimSpace(*c.String())
}

func kvMap(name string, inputs []*core.Connection) map[string]string {
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

// backendEnv merges the free-form credentials key-value input with the typed,
// provider-specific backend auth fields (the latter take precedence).
func backendEnv(inputs []*core.Connection) map[string]string {
	env := kvMap("credentials", inputs)
	if env == nil {
		env = map[string]string{}
	}
	auth := tofu.BackendAuthEnv(optStr("backend_auth", inputs), func(n string) string { return optStr(n, inputs) })
	for k, v := range auth {
		env[k] = v
	}
	return env
}

// backendConfig merges the provider-derived `-backend-config` entries (e.g. the
// GitLab http address/lock plumbing) with the user's explicit backend_config
// input. Explicit entries win so a user can always override the derived values.
func backendConfig(inputs []*core.Connection) map[string]string {
	cfg := tofu.BackendConfigFor(optStr("backend_auth", inputs), func(n string) string { return optStr(n, inputs) })
	if cfg == nil {
		cfg = map[string]string{}
	}
	for k, v := range kvMap("backend_config", inputs) {
		cfg[k] = v
	}
	return cfg
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxOutputBytes {
		return s[:maxOutputBytes] + "\n... (output truncated)"
	}
	return s
}

func errResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"status":      "failed",
		"stdout":      "",
		"stderr":      msg,
		"destroy":     int64(0),
		"exit_code":   int64(-1),
		"success":     false,
	}
}

func failResult(msg string, res *tofu.RunResult) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%s (exit %d)", msg, res.ExitCode),
		"status":      "failed",
		"stdout":      truncate(res.Stdout),
		"stderr":      truncate(res.Stderr),
		"destroy":     int64(0),
		"exit_code":   int64(res.ExitCode),
		"success":     false,
	}
}

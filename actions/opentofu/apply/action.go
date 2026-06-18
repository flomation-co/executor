// Package opentofu_apply applies an OpenTofu configuration. By default it
// pauses for human approval: on first execution it runs init + plan, suspends
// the flow with a summary of the pending changes, and only applies once the
// flow is resumed (e.g. after an operator approves).
//
// State note: because the executor is stateless and a resumed flow may run on a
// different runner, apply re-initialises against the configured (remote)
// backend rather than relying on a saved plan file surviving the suspend. Use a
// remote backend with locking; local state is not safe across resume.
package opentofu_apply

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/opentofu"
	"flomation.app/automate/executor/actions/opentofu/tofu"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "OpenTofu Apply"
	Description  = "Apply an OpenTofu (Terraform-compatible) configuration, optionally pausing for human approval after showing the planned changes."
	Website      = "https://www.flomation.co"
	Icon         = "box+check"
	Date         = "18/06/2026"
	Type         = core.ActionTypeAction

	defaultTimeout = 1800 // seconds
	maxTimeout     = 3600 // seconds

	approvalReason = "opentofu_apply_approval"
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
	{Name: "stdout", Type: core.ConnectionTypeText, Label: "Apply Output"},
	{Name: "stderr", Type: core.ConnectionTypeText, Label: "Standard Error"},
	{Name: "outputs_json", Type: core.ConnectionTypeText, Label: "Outputs (JSON)"},
	{Name: "exit_code", Type: core.ConnectionTypeInteger, Label: "Exit Code"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	workDir, err := opentofu.WorkDir(inputs)
	if err != nil {
		return errResult(err.Error()), nil
	}
	if err := opentofu.CheckBackend(workDir, inputs); err != nil {
		return errResult(err.Error()), nil
	}
	timeout, err := opentofu.Timeout(opentofu.OptStr("timeout_seconds", inputs), defaultTimeout, maxTimeout)
	if err != nil {
		return errResult(err.Error()), nil
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), timeout)
	defer cancel()

	cfg := opentofu.Config(workDir, inputs)

	// Approval gate: on the first pass, plan and suspend. Skipped when approval
	// is disabled or when this node is the one being resumed.
	if opentofu.OptBool("require_approval", inputs, true) && !flow.IsResumedNode(node.ID) {
		return planAndSuspend(ctx, flow, node, cfg)
	}

	return apply(ctx, cfg)
}

// planAndSuspend runs init + plan to summarise the change, then suspends the
// flow for approval. Returns core.ErrSuspended so the engine checkpoints.
func planAndSuspend(ctx context.Context, flow *core.Flow, node *core.Node, cfg tofu.RunConfig) (map[string]interface{}, error) {
	bin, env, initRes, err := tofu.Prepare(ctx, cfg)
	if err != nil {
		return errResult(fmt.Sprintf("tofu init failed to start: %v", err)), nil
	}
	if initRes.ExitCode != 0 {
		return failResult("tofu init failed", initRes), nil
	}

	planRes, err := tofu.Run(ctx, bin, cfg.WorkDir, env, "plan", "-input=false", "-no-color", "-json")
	if err != nil {
		return errResult(fmt.Sprintf("tofu plan failed to run: %v", err)), nil
	}
	if planRes.ExitCode != 0 {
		return failResult("tofu plan failed", planRes), nil
	}

	summary, _ := tofu.ParsePlanSummary(planRes.Stdout)

	flow.Suspend(&core.SuspendInfo{
		NodeID: node.ID,
		Reason: approvalReason,
	})

	r := opentofu.BaseResult("Awaiting approval to apply: "+summary.String(), planRes.Stdout, planRes.Stderr, 0, true)
	r["status"] = "pending_approval"
	r["outputs_json"] = ""
	return r, core.ErrSuspended
}

// apply runs init + apply, then reads the stack outputs.
func apply(ctx context.Context, cfg tofu.RunConfig) (map[string]interface{}, error) {
	bin, env, initRes, err := tofu.Prepare(ctx, cfg)
	if err != nil {
		return errResult(fmt.Sprintf("tofu init failed to start: %v", err)), nil
	}
	if initRes.ExitCode != 0 {
		return failResult("tofu init failed", initRes), nil
	}

	applyRes, err := tofu.Run(ctx, bin, cfg.WorkDir, env, "apply", "-input=false", "-no-color", "-auto-approve")
	if err != nil {
		return errResult(fmt.Sprintf("tofu apply failed to run: %v", err)), nil
	}

	success := applyRes.ExitCode == 0
	outputsJSON := ""
	if success {
		// Best-effort: surface stack outputs for downstream nodes.
		if outRes, oErr := tofu.Run(ctx, bin, cfg.WorkDir, env, "output", "-json", "-no-color"); oErr == nil {
			outputsJSON = opentofu.Truncate(outRes.Stdout)
		}
	}

	toolResult := fmt.Sprintf("Apply failed (exit %d)", applyRes.ExitCode)
	status := "failed"
	if success {
		toolResult = "Apply complete"
		status = "applied"
	}

	r := opentofu.BaseResult(toolResult, applyRes.Stdout, applyRes.Stderr, applyRes.ExitCode, success)
	r["status"] = status
	r["outputs_json"] = outputsJSON
	return r, nil
}

// failExtra is the action-specific output schema in its failure (zero) state.
func failExtra() map[string]interface{} {
	return map[string]interface{}{
		"status":       "failed",
		"outputs_json": "",
	}
}

func errResult(msg string) map[string]interface{} { return opentofu.ErrResult(msg, failExtra()) }

func failResult(msg string, res *tofu.RunResult) map[string]interface{} {
	return opentofu.FailResult(msg, res, failExtra())
}

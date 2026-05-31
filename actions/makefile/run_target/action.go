// Package run_target executes a Make target in a specified directory.
// Supports custom variables, environment variables, and timeout control.
package run_target

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Run Make Target"
	Description  = "Execute a Makefile target"
	Website      = "https://www.flomation.co"
	Icon         = "gears"
	Date         = "31/05/2026"
	Type         = core.ActionTypeAction

	defaultTimeout  = 120
	maxTimeout      = 600
	maxOutputBytes  = 1 << 20 // 1 MB
)

var Inputs = [...]core.Connection{
	{
		Name:        "target",
		Type:        core.ConnectionTypeString,
		Label:       "Target",
		Placeholder: "build",
		Required:    true,
	},
	{
		Name:        "working_directory",
		Type:        core.ConnectionTypeString,
		Label:       "Working Directory",
		Placeholder: "/path/to/project",
	},
	{
		Name:        "makefile_path",
		Type:        core.ConnectionTypeString,
		Label:       "Makefile Path",
		Placeholder: "Makefile",
	},
	{
		Name:        "variables",
		Type:        core.ConnectionTypeString,
		Label:       "Variables",
		Placeholder: "CC=gcc CFLAGS=-O2",
	},
	{
		Name:        "timeout_seconds",
		Type:        core.ConnectionTypeInteger,
		Label:       "Timeout (seconds)",
		Placeholder: "120",
	},
	{
		Name:        "dry_run",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Dry Run",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "stdout", Type: core.ConnectionTypeString, Label: "Standard Output"},
	{Name: "stderr", Type: core.ConnectionTypeString, Label: "Standard Error"},
	{Name: "exit_code", Type: core.ConnectionTypeInteger, Label: "Exit Code"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	target := optStr("target", inputs)
	if target == "" {
		return errResult("target is required", -1)
	}

	workDir := optStr("working_directory", inputs)
	makefilePath := optStr("makefile_path", inputs)
	variables := optStr("variables", inputs)
	dryRun := optStr("dry_run", inputs) == "true"

	timeoutSec := defaultTimeout
	if ts := optStr("timeout_seconds", inputs); ts != "" {
		if v, err := strconv.Atoi(ts); err == nil && v > 0 {
			timeoutSec = v
		}
	}
	if timeoutSec > maxTimeout {
		timeoutSec = maxTimeout
	}

	// Resolve working directory
	if workDir == "" || strings.HasPrefix(workDir, "${") {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return errResult(fmt.Sprintf("unable to determine working directory: %v", err), -1)
		}
	}

	// Build make command arguments
	args := []string{}
	if makefilePath != "" && !strings.HasPrefix(makefilePath, "${") {
		args = append(args, "-f", makefilePath)
	}
	if dryRun {
		args = append(args, "-n")
	}

	// Parse and append variables (KEY=VALUE pairs)
	if variables != "" && !strings.HasPrefix(variables, "${") {
		for _, v := range splitVariables(variables) {
			if v != "" {
				args = append(args, v)
			}
		}
	}

	args = append(args, target)

	// Execute
	ctx, cancel := context.WithTimeout(flow.GoContext(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "make", args...)
	cmd.Dir = workDir
	cmd.Env = buildEnv(workDir)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.WaitDelay = 5 * time.Second

	log.WithFields(log.Fields{
		"target":    target,
		"directory": workDir,
		"args":      args,
	}).Debug("executing make target")

	err := cmd.Run()

	stdoutStr := truncate(stdoutBuf.String(), maxOutputBytes)
	stderrStr := truncate(stderrBuf.String(), maxOutputBytes)
	exitCode := int64(0)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return makeResult(
				fmt.Sprintf("make %s timed out after %ds", target, timeoutSec),
				stdoutStr, stderrStr, -1, false,
			), fmt.Errorf("make %s timed out after %d seconds", target, timeoutSec)
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = int64(exitErr.ExitCode())
		} else {
			exitCode = -1
		}

		return makeResult(
			fmt.Sprintf("make %s failed (exit %d): %s", target, exitCode, firstLine(stderrStr)),
			stdoutStr, stderrStr, exitCode, false,
		), fmt.Errorf("make %s exited with code %d: %s", target, exitCode, firstLine(stderrStr))
	}

	summary := fmt.Sprintf("make %s completed successfully", target)
	if dryRun {
		summary = fmt.Sprintf("make %s (dry run) completed", target)
	}

	return makeResult(summary, stdoutStr, stderrStr, 0, true), nil
}

func makeResult(toolResult, stdout, stderr string, exitCode int64, success bool) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": toolResult,
		"stdout":      strings.TrimSpace(stdout),
		"stderr":      strings.TrimSpace(stderr),
		"exit_code":   exitCode,
		"success":     success,
	}
}

func errResult(msg string, exitCode int64) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"stdout":      "",
		"stderr":      msg,
		"exit_code":   exitCode,
		"success":     false,
	}, nil
}

// splitVariables splits a string of KEY=VALUE pairs, respecting quoted values.
func splitVariables(s string) []string {
	var result []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			current.WriteByte(c)
			if c == quoteChar {
				inQuote = false
			}
		} else if c == '\'' || c == '"' {
			inQuote = true
			quoteChar = c
			current.WriteByte(c)
		} else if c == ' ' || c == '\t' {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

func buildEnv(workDir string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + workDir,
		"LANG=en_GB.UTF-8",
		"TERM=dumb",
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "\n... (output truncated)"
	}
	return s
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}

func optStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

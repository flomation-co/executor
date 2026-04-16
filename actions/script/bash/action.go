package script_bash

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Run Bash Script"
	Description  = "Execute a Bash script in the current working directory"
	Website      = "https://www.flomation.co"
	Icon         = "terminal"
	Date         = "23/03/2026"
	Type         = core.ActionTypeAction

	defaultTimeout = 60
	maxTimeout     = 300
	maxOutputBytes = 1 << 20 // 1 MB
)

var Inputs = [...]core.Connection{
	{
		Name:        "script",
		Type:        core.ConnectionTypeText,
		Label:       "Script",
		Placeholder: "#!/bin/bash\necho \"Hello, World!\"",
		Required:    true,
	},
	{
		Name:        "timeout_seconds",
		Type:        core.ConnectionTypeInteger,
		Label:       "Timeout (seconds)",
		Placeholder: "60",
	},
	{
		Name:  "sandboxed",
		Type:  core.ConnectionTypeBoolean,
		Label: "Run in Sandboxed Directory",
	},
	{
		Name:  "environment_variables",
		Type:  core.ConnectionTypeKeyValueArray,
		Label: "Environment Variables",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{
		Name: "stdout",
		Type: core.ConnectionTypeString,
	},
	{
		Name: "stderr",
		Type: core.ConnectionTypeString,
	},
	{
		Name: "exit_code",
		Type: core.ConnectionTypeInteger,
	},
	{
		Name: "success",
		Type: core.ConnectionTypeBoolean,
	},
	{
		Name: "working_directory",
		Type: core.ConnectionTypeString,
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// 1. Extract and validate script input
	scriptConn := core.FindConnection("script", inputs)
	if scriptConn == nil || scriptConn.String() == nil || strings.TrimSpace(*scriptConn.String()) == "" {
		return nil, fmt.Errorf("script is required")
	}
	scriptContent := *scriptConn.String()

	// 2. Parse timeout
	timeoutSecs := int64(defaultTimeout)
	if tc := core.FindConnection("timeout_seconds", inputs); tc != nil && tc.Number() != nil {
		timeoutSecs = *tc.Number()
		if timeoutSecs <= 0 {
			timeoutSecs = int64(defaultTimeout)
		}
		if timeoutSecs > int64(maxTimeout) {
			timeoutSecs = int64(maxTimeout)
		}
	}

	// 3. Determine working directory
	sandboxed := false
	if sc := core.FindConnection("sandboxed", inputs); sc != nil && sc.String() != nil {
		sandboxed = *sc.String() == "true"
	}

	var workDir string
	if sandboxed {
		tmpDir, err := os.MkdirTemp("", "flomation-bash-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary directory: %w", err)
		}
		workDir = tmpDir
		defer os.RemoveAll(workDir)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		workDir = cwd
	}

	// 4. Write script to temp file
	scriptPath := filepath.Join(workDir, ".flomation-script.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0700); err != nil {
		return nil, fmt.Errorf("failed to write script: %w", err)
	}
	defer os.Remove(scriptPath)

	// 5. Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	// 6. Build command with sandboxed environment
	cmd := exec.CommandContext(ctx, "/bin/bash", scriptPath)
	cmd.Dir = workDir
	cmd.WaitDelay = 5 * time.Second
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + workDir,
		"LANG=en_GB.UTF-8",
	}

	// 6b. Add user-defined environment variables
	if envVars := core.FindConnection("environment_variables", inputs); envVars != nil {
		for _, kv := range envVars.KeyValuePairs() {
			if kv.Key != "" {
				env = append(env, kv.Key+"="+kv.Value)
			}
		}
	}

	cmd.Env = env

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// 7. Execute
	err := cmd.Run()

	// 8. Truncate outputs if too large
	stdoutStr := truncateString(stdoutBuf.String(), maxOutputBytes)
	stderrStr := truncateString(stderrBuf.String(), maxOutputBytes)

	// 9. Determine exit code
	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("script exceeded timeout of %d seconds", timeoutSecs)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to execute script: %w", err)
		}
	}

	if exitCode != 0 {
		return map[string]interface{}{
			"stdout":            strings.TrimSpace(stdoutStr),
			"stderr":            strings.TrimSpace(stderrStr),
			"exit_code":         exitCode,
			"success":           false,
			"working_directory": workDir,
		}, fmt.Errorf("script exited with code %d: %s", exitCode, strings.TrimSpace(stderrStr))
	}

	return map[string]interface{}{
		"stdout":            strings.TrimSpace(stdoutStr),
		"stderr":            strings.TrimSpace(stderrStr),
		"exit_code":         exitCode,
		"success":           true,
		"working_directory": workDir,
	}, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

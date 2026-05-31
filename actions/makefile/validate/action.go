// Package validate checks a Makefile for syntax errors and missing
// dependencies by running `make -n` (dry run) on the default target.
package validate

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
	Name         = "Validate Makefile"
	Description  = "Check a Makefile for syntax errors"
	Website      = "https://www.flomation.co"
	Icon         = "gears"
	Date         = "31/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
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
		Name:        "target",
		Type:        core.ConnectionTypeString,
		Label:       "Target to Validate",
		Placeholder: "all",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "valid", Type: core.ConnectionTypeBoolean, Label: "Valid"},
	{Name: "errors", Type: core.ConnectionTypeString, Label: "Errors"},
	{Name: "commands", Type: core.ConnectionTypeString, Label: "Commands (dry run output)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	workDir := optStr("working_directory", inputs)
	makefilePath := optStr("makefile_path", inputs)
	target := optStr("target", inputs)

	if workDir == "" || strings.HasPrefix(workDir, "${") {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return errResult(fmt.Sprintf("unable to determine working directory: %v", err))
		}
	}

	// Build args: make -n [-f path] [target]
	args := []string{"-n"}
	if makefilePath != "" && !strings.HasPrefix(makefilePath, "${") {
		if !filepath.IsAbs(makefilePath) {
			makefilePath = filepath.Join(workDir, makefilePath)
		}
		args = append(args, "-f", makefilePath)
	}
	if target != "" && !strings.HasPrefix(target, "${") {
		args = append(args, target)
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "make", args...)
	cmd.Dir = workDir
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + workDir,
		"LANG=en_GB.UTF-8",
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.WaitDelay = 5 * time.Second

	err := cmd.Run()

	stdout := strings.TrimSpace(stdoutBuf.String())
	stderr := strings.TrimSpace(stderrBuf.String())

	if err != nil {
		targetDesc := "default target"
		if target != "" {
			targetDesc = fmt.Sprintf("target '%s'", target)
		}

		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Makefile validation failed for %s: %s", targetDesc, firstLine(stderr)),
			"valid":       false,
			"errors":      stderr,
			"commands":    stdout,
			"success":     true, // The action itself succeeded; the Makefile is invalid
		}, nil
	}

	return map[string]interface{}{
		"tool_result": "Makefile is valid",
		"valid":       true,
		"errors":      "",
		"commands":    stdout,
		"success":     true,
	}, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"valid":       false,
		"errors":      msg,
		"commands":    "",
		"success":     false,
	}, nil
}

func optStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

// Package get_variable extracts the value of a variable from a Makefile
// by running `make -p` and parsing the output database.
package get_variable

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"os/exec"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Make Variable"
	Description  = "Extract a variable value from a Makefile"
	Website      = "https://www.flomation.co"
	Icon         = "gears"
	Date         = "31/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "variable",
		Type:        core.ConnectionTypeString,
		Label:       "Variable Name",
		Placeholder: "CC",
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "value", Type: core.ConnectionTypeString, Label: "Variable Value"},
	{Name: "found", Type: core.ConnectionTypeBoolean, Label: "Found"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	variable := optStr("variable", inputs)
	if variable == "" {
		return errResult("variable name is required")
	}

	workDir := optStr("working_directory", inputs)
	makefilePath := optStr("makefile_path", inputs)

	if workDir == "" || strings.HasPrefix(workDir, "${") {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return errResult(fmt.Sprintf("unable to determine working directory: %v", err))
		}
	}

	// Use `make -p -n` to print the database without executing anything.
	// The -n (dry run) prevents any commands from running.
	args := []string{"-p", "-n"}
	if makefilePath != "" && !strings.HasPrefix(makefilePath, "${") {
		if !filepath.IsAbs(makefilePath) {
			makefilePath = filepath.Join(workDir, makefilePath)
		}
		args = append(args, "-f", makefilePath)
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "make", args...)
	cmd.Dir = workDir
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + workDir,
		"LANG=en_GB.UTF-8",
	}

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &bytes.Buffer{} // discard stderr
	cmd.WaitDelay = 5 * time.Second

	// make -p -n may exit non-zero if the default target has no recipe;
	// we still get the variable database in stdout, so ignore the error.
	_ = cmd.Run()

	// Parse the database output for the variable
	value, found := parseVariable(stdoutBuf.String(), variable)

	if !found {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Variable '%s' not found in Makefile", variable),
			"value":       "",
			"found":       false,
			"success":     true,
		}, nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%s = %s", variable, value),
		"value":       value,
		"found":       true,
		"success":     true,
	}, nil
}

// parseVariable searches the make -p output for a variable assignment.
// The format is: VARIABLE = value (or := or ?= etc.)
func parseVariable(output, variable string) (string, bool) {
	// Look for exact match: "VARIABLE = value" or "VARIABLE := value"
	prefix := variable + " "
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}

		rest := line[len(prefix):]
		// Check for assignment operators
		for _, op := range []string{":=", "=", "?=", "+=", "::="} {
			if strings.HasPrefix(rest, op) {
				value := strings.TrimSpace(rest[len(op):])
				return value, true
			}
		}
	}

	return "", false
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"value":       "",
		"found":       false,
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

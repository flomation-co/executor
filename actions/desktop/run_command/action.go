// Package run_command runs an arbitrary shell command on a desktop VM over SSH,
// targeting the desktop session (DISPLAY). This is the reliable alternative to
// GUI clicking: launch apps, set the wallpaper, download files, move/focus
// windows with wmctrl/xdotool, etc. — directly, instead of hunting for buttons.
package run_command

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	desktop "flomation.app/automate/executor/actions/desktop"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Desktop Run Command"
	Description  = "Run a shell command on a desktop VM (targets the desktop session) and return its output."
	Website      = "https://www.flomation.co"
	Icon         = "terminal"
	Date         = "13/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Host", Placeholder: "10.0.0.5", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "22"},
	{Name: "os", Type: core.ConnectionTypeString, Label: "Operating System", Options: []core.ConnectionOption{{Name: "Linux", Value: "linux"}, {Name: "Windows", Value: "windows"}}},
	{Name: "display", Type: core.ConnectionTypeString, Label: "X Display (Linux)", Placeholder: ":0"},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{{Name: "Private Key", Value: "key"}, {Name: "Password", Value: "password"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "SSH Private Key", Placeholder: "${secrets.DesktopVMKey}", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"key"}}},
	{Name: "passphrase", Type: core.ConnectionTypeSecret, Label: "Key Passphrase", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"key"}}},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"password"}}},
	{Name: "host_fingerprint", Type: core.ConnectionTypeString, Label: "Host Key Fingerprint", Placeholder: "SHA256:… (optional but recommended)"},
	{Name: "command", Type: core.ConnectionTypeText, Label: "Command", Placeholder: "wget -qO /tmp/bg.png https://… && xfconf-query -c xfce4-desktop -p …", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "stdout", Type: core.ConnectionTypeString, Label: "Standard Output"},
	{Name: "stderr", Type: core.ConnectionTypeString, Label: "Standard Error"},
	{Name: "exit_code", Type: core.ConnectionTypeInteger, Label: "Exit Code"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// maxSummaryOutput bounds how much command output goes into tool_result (the
// LLM-visible summary). Full stdout/stderr stay in their own outputs.
const maxSummaryOutput = 4000

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	conn, err := desktop.ResolveConn(inputs)
	if err != nil {
		return desktop.ErrResult(err.Error()), nil
	}
	command := desktop.OptionalString("command", inputs)
	if strings.TrimSpace(command) == "" {
		return desktop.ErrResult("command is required"), nil
	}

	stdout, stderr, exit, rerr := conn.Run(desktop.RunCommandCmd(conn.OS, conn.Display, command))
	if rerr != nil {
		return desktop.ErrResult(rerr.Error()), nil
	}

	extra := map[string]interface{}{
		"stdout":    stdout,
		"stderr":    stderr,
		"exit_code": exit,
	}
	if exit != 0 {
		// Non-zero exit is a normal, recoverable outcome the AI should see —
		// return success:false with the diagnostics, not a hard node failure.
		out := desktop.ErrResult(fmt.Sprintf("Command exited %d.%s", exit, summaryTail(stderr, stdout)))
		for k, v := range extra {
			out[k] = v
		}
		return out, nil
	}
	return desktop.OkResult(fmt.Sprintf("Command exited 0.%s", summaryTail(stdout, stderr)), extra), nil
}

// summaryTail appends whichever stream has content (primary first) to the
// tool_result, truncated so a chatty command can't blow the AI's context.
func summaryTail(primary, secondary string) string {
	pick := strings.TrimSpace(primary)
	label := " Output: "
	if pick == "" {
		pick = strings.TrimSpace(secondary)
		label = " stderr: "
	}
	if pick == "" {
		return " (no output)"
	}
	if len(pick) > maxSummaryOutput {
		pick = pick[:maxSummaryOutput] + "… [truncated]"
	}
	return label + pick
}

// Package type_text types literal text into the focused window on a desktop VM.
package type_text

import (
	core "flomation.app/automate/executor"
	desktop "flomation.app/automate/executor/actions/desktop"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Desktop Type Text"
	Description  = "Type literal text into the focused window on a desktop VM."
	Website      = "https://www.flomation.co"
	Icon         = "i-cursor"
	Date         = "12/08/2026"
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
	{Name: "text", Type: core.ConnectionTypeText, Label: "Text", Placeholder: "Text to type into the focused field", Required: true},
	{Name: "window", Type: core.ConnectionTypeString, Label: "Focus Window First (title substring, optional)", Placeholder: "Google Chrome"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	conn, err := desktop.ResolveConn(inputs)
	if err != nil {
		return desktop.ErrResult(err.Error()), nil
	}
	text := desktop.OptionalString("text", inputs)
	if text == "" {
		return desktop.ErrResult("text is required"), nil
	}
	// Optionally focus a target window first so the text lands in it.
	conn.FocusWindowIfRequested(inputs)
	if _, stderr, exit, rerr := conn.Run(desktop.TypeCmd(conn.OS, conn.Display, text)); rerr != nil {
		return desktop.ErrResult(rerr.Error()), nil
	} else if exit != 0 {
		return desktop.ErrResult("type failed: " + stderr), nil
	}
	return desktop.OkResult("Typed text into the focused window", nil), nil
}

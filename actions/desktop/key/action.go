// Package key presses a key or key chord on a desktop VM.
package key

import (
	core "flomation.app/automate/executor"
	desktop "flomation.app/automate/executor/actions/desktop"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Desktop Press Key"
	Description  = "Press a key or chord on a desktop VM (e.g. Return, ctrl+c / ^c)."
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
	{Name: "keys", Type: core.ConnectionTypeString, Label: "Key(s)", Placeholder: "Linux: Return, ctrl+c · Windows: {ENTER}, ^c", Required: true},
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
	keys := desktop.OptionalString("keys", inputs)
	if keys == "" {
		return desktop.ErrResult("keys is required"), nil
	}
	if _, stderr, exit, rerr := conn.Run(desktop.KeyCmd(conn.OS, conn.Display, keys)); rerr != nil {
		return desktop.ErrResult(rerr.Error()), nil
	} else if exit != 0 {
		return desktop.ErrResult("key press failed: " + stderr), nil
	}
	return desktop.OkResult("Pressed "+keys, nil), nil
}

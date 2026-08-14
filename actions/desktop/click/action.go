// Package click moves the pointer to a coordinate on a desktop VM and clicks.
package click

import (
	"fmt"

	core "flomation.app/automate/executor"
	desktop "flomation.app/automate/executor/actions/desktop"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Desktop Click"
	Description  = "Move the mouse to (x, y) on a desktop VM and click a button."
	Website      = "https://www.flomation.co"
	Icon         = "hand"
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
	{Name: "x", Type: core.ConnectionTypeInteger, Label: "X", Required: true},
	{Name: "y", Type: core.ConnectionTypeInteger, Label: "Y", Required: true},
	{Name: "button", Type: core.ConnectionTypeString, Label: "Button", Options: []core.ConnectionOption{{Name: "Left", Value: "left"}, {Name: "Right", Value: "right"}, {Name: "Middle", Value: "middle"}}},
	{Name: "clicks", Type: core.ConnectionTypeInteger, Label: "Clicks (1 = single, 2 = double)", Placeholder: "1"},
	{Name: "window", Type: core.ConnectionTypeString, Label: "Focus Window First (title substring, optional)", Placeholder: "Google Chrome"},
	{Name: "settle_ms", Type: core.ConnectionTypeInteger, Label: "Settle After (ms)", Placeholder: "300"},
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
	x := desktop.Int("x", inputs, 0)
	y := desktop.Int("y", inputs, 0)
	button := desktop.OptionalString("button", inputs)
	if button == "" {
		button = "left"
	}
	clicks := desktop.Int("clicks", inputs, 1)
	if clicks < 1 {
		clicks = 1
	}
	// Optionally raise+focus a target window first, so the click lands on it
	// rather than whatever is on top (e.g. a leftover terminal).
	conn.FocusWindowIfRequested(inputs)

	if _, stderr, exit, rerr := conn.Run(desktop.ClickCmd(conn.OS, conn.Display, x, y, button, clicks)); rerr != nil {
		return desktop.ErrResult(rerr.Error()), nil
	} else if exit != 0 {
		return desktop.ErrResult("click failed: " + stderr), nil
	}
	desktop.SettleAfter(inputs)

	summary := fmt.Sprintf("%s-clicked at (%d, %d)", button, x, y)
	switch {
	case clicks == 2:
		summary = fmt.Sprintf("%s double-clicked at (%d, %d)", button, x, y)
	case clicks > 2:
		summary = fmt.Sprintf("%s-clicked %d times at (%d, %d)", button, clicks, x, y)
	}
	return desktop.OkResult(summary, nil), nil
}

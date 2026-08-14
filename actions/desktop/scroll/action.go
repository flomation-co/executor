// Package scroll scrolls the mouse wheel up or down on a desktop VM.
package scroll

import (
	"fmt"

	core "flomation.app/automate/executor"
	desktop "flomation.app/automate/executor/actions/desktop"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Desktop Scroll"
	Description  = "Scroll the mouse wheel up or down on a desktop VM."
	Website      = "https://www.flomation.co"
	Icon         = "arrows-up-down"
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
	{Name: "direction", Type: core.ConnectionTypeString, Label: "Direction", Options: []core.ConnectionOption{{Name: "Down", Value: "down"}, {Name: "Up", Value: "up"}}},
	{Name: "amount", Type: core.ConnectionTypeInteger, Label: "Notches", Placeholder: "3"},
	{Name: "x", Type: core.ConnectionTypeInteger, Label: "X (point at the area to scroll)"},
	{Name: "y", Type: core.ConnectionTypeInteger, Label: "Y (point at the area to scroll)"},
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
	direction := desktop.OptionalString("direction", inputs)
	if direction == "" {
		direction = "down"
	}
	amount := desktop.Int("amount", inputs, 3)
	at := desktop.PointFrom(inputs, "x", "y")

	conn.FocusWindowIfRequested(inputs)

	if _, stderr, exit, rerr := conn.Run(desktop.ScrollCmd(conn.OS, conn.Display, direction, amount, at)); rerr != nil {
		return desktop.ErrResult(rerr.Error()), nil
	} else if exit != 0 {
		return desktop.ErrResult("scroll failed: " + stderr), nil
	}
	desktop.SettleAfter(inputs)

	where := "at the current pointer position"
	if at != nil {
		where = fmt.Sprintf("at (%d, %d)", at.X, at.Y)
	}
	return desktop.OkResult(fmt.Sprintf("Scrolled %s by %d %s", direction, amount, where), nil), nil
}

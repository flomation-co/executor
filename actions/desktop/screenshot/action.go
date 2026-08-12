// Package screenshot captures the screen of a desktop VM over SSH and returns
// the image as a blob reference (wire it into an AI vision node or Slack upload).
package screenshot

import (
	"fmt"
	"os"

	core "flomation.app/automate/executor"
	desktop "flomation.app/automate/executor/actions/desktop"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Desktop Screenshot"
	Description  = "Capture the screen of a desktop VM over SSH and return the image."
	Website      = "https://www.flomation.co"
	Icon         = "image"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Screenshot (image reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	conn, err := desktop.ResolveConn(inputs)
	if err != nil {
		return desktop.ErrResult(err.Error()), nil
	}

	path := desktop.ScreenshotPath(conn.OS)
	if _, stderr, exit, rerr := conn.Run(desktop.ScreenshotCmd(conn.OS, conn.Display, path)); rerr != nil {
		return desktop.ErrResult(rerr.Error()), nil
	} else if exit != 0 {
		return desktop.ErrResult("screenshot command failed: " + stderr), nil
	}

	img, err := conn.ReadFileBytes(path)
	if err != nil {
		return desktop.ErrResult(err.Error()), nil
	}
	if len(img) == 0 {
		return desktop.ErrResult("screenshot was empty — is an interactive desktop session available?"), nil
	}

	scratch, err := flow.MediaScratchFile("png")
	if err != nil {
		return desktop.ErrResult("allocating scratch file: " + err.Error()), nil
	}
	if err := os.WriteFile(scratch, img, 0o600); err != nil {
		return desktop.ErrResult("writing screenshot: " + err.Error()), nil
	}
	ref, err := flow.EmitMediaFile(scratch)
	if err != nil {
		return desktop.ErrResult("emitting screenshot: " + err.Error()), nil
	}

	return desktop.OkResult(
		fmt.Sprintf("Captured screenshot (%d bytes). Pass this to a vision node or upload action's file input: %s", len(img), ref),
		map[string]interface{}{"image": ref, "size_bytes": int64(len(img))},
	), nil
}

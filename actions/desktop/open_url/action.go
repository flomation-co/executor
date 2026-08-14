// Package open_url opens a URL in the default browser on a desktop VM.
package open_url

import (
	core "flomation.app/automate/executor"
	desktop "flomation.app/automate/executor/actions/desktop"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Desktop Open URL"
	Description  = "Open a URL in the default browser on a desktop VM."
	Website      = "https://www.flomation.co"
	Icon         = "globe"
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
	{Name: "url", Type: core.ConnectionTypeString, Label: "URL", Placeholder: "https://www.flomation.co", Required: true},
	{Name: "same_tab", Type: core.ConnectionTypeBoolean, Label: "Navigate the current tab (instead of opening a new one)"},
	{Name: "window", Type: core.ConnectionTypeString, Label: "Browser Window (title substring, used with Navigate current tab)", Placeholder: "Google Chrome", Visible: &core.VisibleWhen{Field: "same_tab", Values: []string{"true"}}},
	{Name: "settle_ms", Type: core.ConnectionTypeInteger, Label: "Settle After (ms)", Placeholder: "1500"},
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
	url := desktop.OptionalString("url", inputs)
	if url == "" {
		return desktop.ErrResult("url is required"), nil
	}

	// Launching the browser binary hands the URL to an already-running instance
	// as a new tab. same_tab drives the address bar of the window that is
	// already open instead, which is what a user journey actually looks like —
	// and keeps "the page" unambiguous for whatever acts on it next.
	sameTab := desktop.OptionalBool("same_tab", inputs)

	cmd := desktop.OpenURLCmd(conn.OS, conn.Display, url)
	summary := "Opened " + url + " in a new tab"
	if sameTab {
		conn.FocusWindowIfRequested(inputs)
		cmd = desktop.NavigateCmd(conn.OS, conn.Display, url)
		summary = "Navigated the current tab to " + url
	}

	if _, stderr, exit, rerr := conn.Run(cmd); rerr != nil {
		return desktop.ErrResult(rerr.Error()), nil
	} else if exit != 0 {
		return desktop.ErrResult("open url failed: " + stderr), nil
	}
	desktop.SettleAfter(inputs)

	return desktop.OkResult(summary, nil), nil
}

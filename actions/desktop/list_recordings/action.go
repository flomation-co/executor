// Package list_recordings lists the ids of screen recordings currently running
// on a desktop VM. Lets an agent discover an active recording's id (e.g. after
// its conversation was compacted and the id from Start Recording was lost) so it
// can stop the right one.
package list_recordings

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	desktop "flomation.app/automate/executor/actions/desktop"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Desktop List Recordings"
	Description  = "List the ids of screen recordings currently running on a desktop VM."
	Website      = "https://www.flomation.co"
	Icon         = "film"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "recordings", Type: core.ConnectionTypeObject, Label: "Active recording ids (newest first)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	conn, err := desktop.ResolveConn(inputs)
	if err != nil {
		return desktop.ErrResult(err.Error()), nil
	}

	stdout, stderr, exit, rerr := conn.Run(desktop.ListRecordingIDsCmd(conn.OS))
	if rerr != nil {
		return desktop.ErrResult(rerr.Error()), nil
	}
	if exit != 0 {
		return desktop.ErrResult("failed to list recordings: " + stderr), nil
	}

	ids := make([]interface{}, 0)
	for _, line := range strings.Split(stdout, "\n") {
		if id := desktop.SafeRecordingID(line); id != "" {
			ids = append(ids, id)
		}
	}

	summary := "No recordings are currently running."
	if len(ids) > 0 {
		strs := make([]string, len(ids))
		for i, v := range ids {
			strs[i] = v.(string)
		}
		summary = fmt.Sprintf("%d recording(s) running (newest first): %s", len(ids), strings.Join(strs, ", "))
	}

	return desktop.OkResult(summary, map[string]interface{}{
		"recordings": ids,
		"count":      int64(len(ids)),
	}), nil
}

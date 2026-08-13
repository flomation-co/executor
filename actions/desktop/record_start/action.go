// Package record_start begins a detached screen recording on a desktop VM
// (ffmpeg). Pair with Desktop Stop Recording to finish and retrieve the video.
package record_start

import (
	"fmt"

	core "flomation.app/automate/executor"
	desktop "flomation.app/automate/executor/actions/desktop"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Desktop Start Recording"
	Description  = "Start recording the screen of a desktop VM (ffmpeg). Stop with Desktop Stop Recording."
	Website      = "https://www.flomation.co"
	Icon         = "video"
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
	{Name: "fps", Type: core.ConnectionTypeInteger, Label: "Frame Rate", Placeholder: "15"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "recording_id", Type: core.ConnectionTypeString, Label: "Recording ID — pass this to Desktop Stop Recording"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	conn, err := desktop.ResolveConn(inputs)
	if err != nil {
		return desktop.ErrResult(err.Error()), nil
	}
	fps := desktop.Int("fps", inputs, 15)

	// A unique id per recording gives each recorder its own pidfile + output
	// file, so multiple recordings can run at once without clobbering each
	// other, and Stop Recording can target exactly this one.
	recordingID := desktop.NewRecordingID()

	stdout, stderr, exit, rerr := conn.Run(desktop.RecordStartCmd(conn.OS, conn.Display, fps, recordingID))
	if rerr != nil {
		return desktop.ErrResult(rerr.Error()), nil
	}
	if exit != 0 {
		return desktop.ErrResult("failed to start recording: " + stderr), nil
	}
	return desktop.OkResult(
		fmt.Sprintf("Recording %s started at %d fps (pid %s). Stop it with Desktop Stop Recording (pass recording_id=%s) to retrieve the video.", recordingID, fps, trim(stdout), recordingID),
		map[string]interface{}{"recording_id": recordingID},
	), nil
}

func trim(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r != '\n' && r != '\r' && r != ' ' && r != '\t' {
			out = append(out, r)
		}
	}
	return string(out)
}

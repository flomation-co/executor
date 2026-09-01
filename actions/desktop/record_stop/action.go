// Package record_stop stops the screen recording started by Desktop Start
// Recording and returns the video as a file reference.
package record_stop

import (
	"fmt"
	"os"

	core "flomation.app/automate/executor"
	desktop "flomation.app/automate/executor/actions/desktop"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Desktop Stop Recording"
	Description  = "Stop a desktop VM screen recording and return the video file."
	Website      = "https://www.flomation.co"
	Icon         = "circle-stop"
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
	{Name: "recording_id", Type: core.ConnectionTypeString, Label: "Recording ID — the id from Desktop Start Recording. Leave blank to stop the most recent recording.", Placeholder: "${recording_id}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Recording (file reference)"},
	{Name: "recording_id", Type: core.ConnectionTypeString, Label: "Recording ID that was stopped"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	conn, err := desktop.ResolveConn(inputs)
	if err != nil {
		return desktop.ErrResult(err.Error()), nil
	}

	recordingID := desktop.SafeRecordingID(desktop.OptionalString("recording_id", inputs))
	if recordingID == "" {
		// No id supplied (e.g. it was lost when the agent's conversation was
		// compacted) — fall back to the most-recently-started running recorder,
		// so a recording is always stoppable.
		out, _, _, rerr := conn.Run(desktop.NewestRecordingIDCmd(conn.OS))
		if rerr != nil {
			return desktop.ErrResult(rerr.Error()), nil
		}
		recordingID = desktop.SafeRecordingID(out)
		if recordingID == "" {
			return desktop.ErrResult("no active recording found to stop"), nil
		}
	}

	// Signal this recording's ffmpeg to stop and wait for it to finalise the file.
	if _, stderr, exit, rerr := conn.Run(desktop.RecordStopCmd(conn.OS, recordingID)); rerr != nil {
		return desktop.ErrResult(rerr.Error()), nil
	} else if exit != 0 {
		return desktop.ErrResult("failed to stop recording: " + stderr), nil
	}

	video, err := conn.ReadFileBytes(desktop.RecordPath(conn.OS, recordingID))
	if err != nil {
		return desktop.ErrResult("retrieving recording: " + err.Error()), nil
	}
	if len(video) == 0 {
		return desktop.ErrResult("recording was empty — was a recording running?"), nil
	}

	scratch, err := flow.MediaScratchFileNamed("recording.mp4")
	if err != nil {
		return desktop.ErrResult("allocating scratch file: " + err.Error()), nil
	}
	if err := os.WriteFile(scratch, video, 0o600); err != nil {
		return desktop.ErrResult("writing recording: " + err.Error()), nil
	}
	ref, err := flow.EmitMediaFile(scratch)
	if err != nil {
		return desktop.ErrResult("emitting recording: " + err.Error()), nil
	}

	return desktop.OkResult(
		fmt.Sprintf("Recording %s stopped (%d bytes). Pass this to an upload/Push-to-S3 action's file input: %s", recordingID, len(video), ref),
		map[string]interface{}{"video": ref, "size_bytes": int64(len(video)), "recording_id": recordingID},
	), nil
}

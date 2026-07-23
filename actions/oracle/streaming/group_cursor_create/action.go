// Package oracle_streaming_group_cursor_create creates a consumer-group cursor. A group cursor
// coordinates consumption across every instance in a named consumer group: the service tracks
// each group's committed position, so instances share the stream's partitions and each message is
// delivered to the group once. Supply the stream OCID, a group name and a starting position
// (LATEST for only new messages, TRIM_HORIZON for the oldest retained, or AT_TIME with a
// timestamp). Returns a cursor to feed straight into Consume Messages. The stream's messages
// endpoint is resolved automatically from its OCID, so no raw endpoint URL is ever needed.
package oracle_streaming_group_cursor_create

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Create Group Cursor"
	Description  = "Create a consumer-group cursor for a stream. Give it a group name and a starting position (LATEST, TRIM_HORIZON, or AT_TIME with a timestamp); the group shares the stream's partitions and tracks its own committed position. Returns a cursor to feed into Consume Messages. The stream's endpoint is resolved automatically."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tower-broadcast"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "stream_ocid", Type: core.ConnectionTypeString, Label: "Stream OCID", Placeholder: "ocid1.stream.oc1..aaaa…", Required: true},
	{Name: "group_name", Type: core.ConnectionTypeString, Label: "Group Name", Placeholder: "Name of the consumer group, e.g. billing-workers", Required: true},
	{Name: "cursor_type", Type: core.ConnectionTypeString, Label: "Cursor Type", Placeholder: "Where to start consuming", Required: true, Options: []core.ConnectionOption{
		{Name: "Latest — only messages published from now on", Value: "LATEST"},
		{Name: "Trim horizon — the oldest retained message", Value: "TRIM_HORIZON"},
		{Name: "At time — from a specific timestamp (set Time)", Value: "AT_TIME"},
	}},
	{Name: "instance_name", Type: core.ConnectionTypeString, Label: "Instance Name", Placeholder: "Unique id for this instance in the group (optional — a UUID is generated)"},
	{Name: "timeout_ms", Type: core.ConnectionTypeString, Label: "Timeout (ms)", Placeholder: "Inactivity before partition reservations are released (optional)"},
	{Name: "commit_on_get", Type: core.ConnectionTypeBoolean, Label: "Commit On Get", Placeholder: "Auto-commit each read (default true; set false to commit manually)"},
	{Name: "time", Type: core.ConnectionTypeString, Label: "Time (AT_TIME only)", Placeholder: "RFC3339, e.g. 2026-08-01T02:00:00Z — required when Cursor Type is AT_TIME"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "cursor", Type: core.ConnectionTypeString, Label: "Cursor"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	streamID, err := str.RequiredString("stream_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	groupName, err := str.RequiredString("group_name", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	cursorType, err := str.RequiredString("cursor_type", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	cursorType = strings.ToUpper(strings.TrimSpace(cursorType))
	switch cursorType {
	case "AT_TIME", "LATEST", "TRIM_HORIZON":
	default:
		return str.ErrorResult("cursor type must be AT_TIME, LATEST or TRIM_HORIZON"), nil
	}

	details := streaming.CreateGroupCursorDetails{
		Type:      streaming.CreateGroupCursorDetailsTypeEnum(cursorType),
		GroupName: &groupName,
	}
	if instanceName := str.OptionalString("instance_name", inputs); instanceName != "" {
		details.InstanceName = &instanceName
	}
	if n, ok, err := str.OptionalInt("timeout_ms", inputs); err != nil {
		return str.ErrorResult(err.Error()), nil
	} else if ok {
		details.TimeoutInMs = &n
	}
	// CommitOnGet defaults to true server-side; only override it when the operator actually set the
	// checkbox, so an untouched field never silently disables auto-commit.
	if c := core.FindConnection("commit_on_get", inputs); c != nil {
		if b := c.Boolean(); b != nil {
			v := *b
			details.CommitOnGet = &v
		}
	}
	// Time is only meaningful for AT_TIME. Require it there, and reject it elsewhere so a stray
	// timestamp doesn't quietly do nothing.
	timeRaw := strings.TrimSpace(str.OptionalString("time", inputs))
	if cursorType == "AT_TIME" {
		if timeRaw == "" {
			return str.ErrorResult("time is required when cursor type is AT_TIME (RFC3339, e.g. 2026-08-01T02:00:00Z)"), nil
		}
		t, err := time.Parse(time.RFC3339, timeRaw)
		if err != nil {
			return str.ErrorResult("time must be an RFC3339 timestamp, e.g. 2026-08-01T02:00:00Z"), nil
		}
		sdkTime := common.SDKTime{Time: t}
		details.Time = &sdkTime
	} else if timeRaw != "" {
		return str.ErrorResult("time only applies when cursor type is AT_TIME"), nil
	}

	auth, client, errResult := str.DataPlaneClientForStream(inputs, streamID)
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.CreateGroupCursor(str.Context(), streaming.CreateGroupCursorRequest{
		StreamId:                 &streamID,
		CreateGroupCursorDetails: details,
	})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}

	return str.Result(fmt.Sprintf("Created group cursor for group %q (%s)", groupName, cursorType), map[string]interface{}{
		"cursor": str.Str(resp.Value),
	}), nil
}

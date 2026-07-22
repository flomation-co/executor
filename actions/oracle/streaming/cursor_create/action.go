// Package oracle_streaming_cursor_create creates a partition cursor — the starting position a
// consumer reads from. A cursor pins one partition of a stream at a point determined by its type:
// TRIM_HORIZON (oldest retained), LATEST (only new messages), AT_OFFSET / AFTER_OFFSET (a specific
// offset) or AT_TIME (a moment in time). Feed the returned cursor straight into Consume Messages.
// The stream's messages endpoint is resolved automatically from its OCID.
package oracle_streaming_cursor_create

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	ocicommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Create Cursor"
	Description  = "Create a partition cursor marking where a consumer starts reading — the oldest retained message (TRIM_HORIZON), only new messages (LATEST), a specific offset (AT_OFFSET / AFTER_OFFSET) or a point in time (AT_TIME). Pass the returned cursor into Consume Messages."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tower-broadcast"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "stream_ocid", Type: core.ConnectionTypeString, Label: "Stream OCID", Placeholder: "ocid1.stream.oc1..aaaa…", Required: true},
	{Name: "partition", Type: core.ConnectionTypeString, Label: "Partition", Placeholder: "The partition to read from, e.g. 0", Required: true},
	{Name: "cursor_type", Type: core.ConnectionTypeString, Label: "Cursor Type", Placeholder: "Where to start reading", Required: true, Options: []core.ConnectionOption{
		{Name: "Trim Horizon (oldest retained)", Value: "TRIM_HORIZON"},
		{Name: "Latest (only new messages)", Value: "LATEST"},
		{Name: "At Offset", Value: "AT_OFFSET"},
		{Name: "After Offset", Value: "AFTER_OFFSET"},
		{Name: "At Time", Value: "AT_TIME"},
	}},
	{Name: "offset", Type: core.ConnectionTypeString, Label: "Offset", Placeholder: "Required for At Offset / After Offset, e.g. 42"},
	{Name: "time", Type: core.ConnectionTypeString, Label: "Time (RFC 3339)", Placeholder: "Required for At Time, e.g. 2026-07-22T09:00:00Z"},
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
	partition, err := str.RequiredString("partition", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	cursorType, err := str.RequiredString("cursor_type", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}

	details := streaming.CreateCursorDetails{
		Partition: &partition,
		Type:      streaming.CreateCursorDetailsTypeEnum(cursorType),
	}

	// Offset applies only to the offset-based cursor types.
	if n, ok, err := str.OptionalInt("offset", inputs); err != nil {
		return str.ErrorResult(err.Error()), nil
	} else if ok {
		off := int64(n)
		details.Offset = &off
	} else if cursorType == "AT_OFFSET" || cursorType == "AFTER_OFFSET" {
		return str.ErrorResult(fmt.Sprintf("offset is required for cursor type %s", cursorType)), nil
	}

	// Time applies only to AT_TIME; parse it as RFC 3339.
	if raw := str.OptionalString("time", inputs); raw != "" {
		t, perr := time.Parse(time.RFC3339, raw)
		if perr != nil {
			return str.ErrorResult("time must be an RFC 3339 timestamp, e.g. 2026-07-22T09:00:00Z"), nil
		}
		details.Time = &ocicommon.SDKTime{Time: t}
	} else if cursorType == "AT_TIME" {
		return str.ErrorResult("time is required for cursor type AT_TIME"), nil
	}

	auth, client, errResult := str.DataPlaneClientForStream(inputs, streamID)
	if errResult != nil {
		return errResult, nil
	}

	resp, err := client.CreateCursor(str.Context(), streaming.CreateCursorRequest{
		StreamId:            &streamID,
		CreateCursorDetails: details,
	})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}

	cursor := str.Str(resp.Value)
	return str.Result(fmt.Sprintf("Created %s cursor for partition %s", cursorType, partition), map[string]interface{}{
		"cursor": cursor,
	}), nil
}

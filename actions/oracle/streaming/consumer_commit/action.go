// Package oracle_streaming_consumer_commit commits a consumer group's cursor offsets. After a
// group has consumed messages (via a group cursor from Create Group Cursor), committing records
// how far the group has read so a later reader resumes from that point rather than replaying. The
// operator supplies the stream OCID and the group cursor; the action returns the committed cursor
// to reuse on the next read. The stream's messages endpoint is resolved automatically from its OCID.
package oracle_streaming_consumer_commit

import (
	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Commit Consumer Offsets"
	Description  = "Commit a consumer group's cursor offsets so a later reader resumes where the group left off instead of replaying. Supply the stream OCID and the group cursor (from Create Group Cursor); the committed cursor is returned to reuse. The stream's endpoint is resolved automatically."
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
	{Name: "cursor", Type: core.ConnectionTypeText, Label: "Group Cursor", Placeholder: "The group cursor to commit (from Create Group Cursor)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "cursor", Type: core.ConnectionTypeString, Label: "Committed Cursor"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	streamID, err := str.RequiredString("stream_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	cursor, err := str.RequiredString("cursor", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := str.DataPlaneClientForStream(inputs, streamID)
	if errResult != nil {
		return errResult, nil
	}

	resp, err := client.ConsumerCommit(str.Context(), streaming.ConsumerCommitRequest{
		StreamId: &streamID,
		Cursor:   &cursor,
	})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}

	return str.Result("Committed consumer group offsets", map[string]interface{}{
		"cursor": str.Str(resp.Value),
	}), nil
}

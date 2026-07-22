// Package oracle_streaming_message_get consumes a batch of messages from a stream using a cursor.
// A cursor marks a position in a partition (or, for a group cursor, across the stream) and is
// obtained from Create Cursor or Create Group Cursor. Each call returns up to `limit` messages
// plus a next_cursor to pass straight into the following call — that is how you page forward
// through a partition. The stream's messages endpoint is resolved automatically from its OCID.
package oracle_streaming_message_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Consume Messages"
	Description  = "Read a batch of messages from a stream using a cursor (from Create Cursor or Create Group Cursor). Returns the messages plus a next cursor to feed into the following call to page forward. The stream's endpoint is resolved automatically."
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
	{Name: "cursor", Type: core.ConnectionTypeText, Label: "Cursor", Placeholder: "A cursor value from Create Cursor / Create Group Cursor", Required: true},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max messages to return, 1–10000 (default service max)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "messages", Type: core.ConnectionTypeObject, Label: "Messages"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Message count"},
	{Name: "next_cursor", Type: core.ConnectionTypeString, Label: "Next Cursor"},
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

	req := streaming.GetMessagesRequest{StreamId: &streamID, Cursor: &cursor}
	if n, ok, err := str.OptionalInt("limit", inputs); err != nil {
		return str.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &n
	}
	resp, err := client.GetMessages(str.Context(), req)
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}

	messages := make([]map[string]interface{}, 0, len(resp.Items))
	for i := range resp.Items {
		messages = append(messages, str.SummariseMessage(&resp.Items[i]))
	}
	return str.Result(fmt.Sprintf("Consumed %d message(s)", len(messages)), map[string]interface{}{
		"messages":    messages,
		"count":       len(messages),
		"next_cursor": str.Str(resp.OpcNextCursor),
	}), nil
}

// Package oracle_streaming_message_put publishes a message to a stream. The operator supplies
// the stream OCID and a value (and optional key); this action resolves the stream's own messages
// endpoint automatically and posts to it, so no raw endpoint URL is ever needed. A key pins the
// message to a partition (all messages with the same key land on the same partition, preserving
// order); with no key the service load-balances across partitions.
package oracle_streaming_message_put

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Publish Message"
	Description  = "Publish a message to a stream. Supply the stream OCID and a value; an optional key pins related messages to the same partition to preserve their order. The stream's endpoint is resolved automatically."
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
	{Name: "value", Type: core.ConnectionTypeText, Label: "Message Value", Placeholder: "The message body (any UTF-8 text or JSON)", Required: true},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Message Key", Placeholder: "Optional partition key — same key ⇒ same partition (ordered)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "partition", Type: core.ConnectionTypeString, Label: "Partition"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Offset"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Timestamp"},
	{Name: "failures", Type: core.ConnectionTypeInteger, Label: "Failures"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Per-message results"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	streamID, err := str.RequiredString("stream_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	// Read the payload WITHOUT trimming — a message value is an opaque byte array (the SDK marks
	// it mandatory but does not normalise it), so leading/trailing whitespace and a trailing
	// newline are significant and must survive verbatim. Only an entirely empty body is rejected.
	value := str.OptionalString("value", inputs)
	if value == "" {
		return str.ErrorResult("Message Value is required"), nil
	}
	auth, client, errResult := str.DataPlaneClientForStream(inputs, streamID)
	if errResult != nil {
		return errResult, nil
	}

	entry := streaming.PutMessagesDetailsEntry{Value: []byte(value)}
	if key := str.OptionalString("key", inputs); key != "" {
		entry.Key = []byte(key)
	}
	resp, err := client.PutMessages(str.Context(), streaming.PutMessagesRequest{
		StreamId:           &streamID,
		PutMessagesDetails: streaming.PutMessagesDetails{Messages: []streaming.PutMessagesDetailsEntry{entry}},
	})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}

	results := make([]map[string]interface{}, 0, len(resp.Entries))
	for i := range resp.Entries {
		results = append(results, str.SummarisePutResult(&resp.Entries[i]))
	}
	failures := str.IntOrNil(resp.Failures)
	// A per-entry error (e.g. the partition is throttled) is reported in the entry, not as an
	// HTTP error — surface it as a soft failure so the operator sees it rather than a silent drop.
	if len(resp.Entries) > 0 && str.Str(resp.Entries[0].Error) != "" {
		return str.Result(fmt.Sprintf("Message rejected: %s", str.Str(resp.Entries[0].ErrorMessage)), map[string]interface{}{
			"partition": results[0]["partition"], "offset": results[0]["offset"], "timestamp": results[0]["timestamp"],
			"failures": failures, "results": results, "success": false, "error": str.Str(resp.Entries[0].ErrorMessage),
		}), nil
	}
	first := map[string]interface{}{}
	if len(results) > 0 {
		first = results[0]
	}
	return str.Result(fmt.Sprintf("Published to partition %v at offset %v", first["partition"], first["offset"]), map[string]interface{}{
		"partition": first["partition"],
		"offset":    first["offset"],
		"timestamp": first["timestamp"],
		"failures":  failures,
		"results":   results,
	}), nil
}

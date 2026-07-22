// Package oracle_streaming_stream_get reads one stream by OCID — its lifecycle state, partition
// count, retention and, once ACTIVE, its messages endpoint (the host the data-plane actions
// publish to and consume from).
package oracle_streaming_stream_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Get Stream"
	Description  = "Read a stream by OCID — its lifecycle state, partitions, retention and messages endpoint."
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "stream", Type: core.ConnectionTypeObject, Label: "Stream"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Stream OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "messages_endpoint", Type: core.ConnectionTypeString, Label: "Messages Endpoint"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := str.AdminClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := str.RequiredString("stream_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetStream(str.Context(), streaming.GetStreamRequest{StreamId: &id})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}
	stream := str.SummariseStream(&resp.Stream)
	return str.Result(fmt.Sprintf("Stream %q is %s", stream["name"], stream["lifecycle_state"]), map[string]interface{}{
		"stream":            stream,
		"id":                stream["id"],
		"lifecycle_state":   stream["lifecycle_state"],
		"messages_endpoint": stream["messages_endpoint"],
	}), nil
}

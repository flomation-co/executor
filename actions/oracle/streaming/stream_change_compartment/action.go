// Package oracle_streaming_stream_change_compartment moves a stream into a different compartment.
// The operator supplies the stream OCID and the destination compartment OCID; the move is
// asynchronous, so the response carries a work request OCID you can poll for completion.
package oracle_streaming_stream_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Move Stream to Compartment"
	Description  = "Move a stream into a different compartment. Supply the stream OCID and the destination compartment OCID. The move is asynchronous and returns a work request OCID to poll."
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
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where the stream should move to)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Stream OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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
	destination, err := str.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}

	resp, err := client.ChangeStreamCompartment(str.Context(), streaming.ChangeStreamCompartmentRequest{
		StreamId:                       &id,
		ChangeStreamCompartmentDetails: streaming.ChangeStreamCompartmentDetails{CompartmentId: &destination},
	})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}
	return str.Result(fmt.Sprintf("Moving stream %q to compartment %q", id, destination), map[string]interface{}{
		"id":                         id,
		"destination_compartment_id": destination,
		"work_request_id":            str.Str(resp.OpcWorkRequestId),
	}), nil
}

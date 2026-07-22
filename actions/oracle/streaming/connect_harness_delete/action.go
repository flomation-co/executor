// Package oracle_streaming_connect_harness_delete deletes a connect harness by OCID. A connect
// harness is the Kafka Connect endpoint that fronts a stream; removing it tears down that
// integration point. Asynchronous: the call returns a work request OCID — poll Get Work Request
// until it succeeds to confirm the harness is gone.
package oracle_streaming_connect_harness_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Delete Connect Harness"
	Description  = "Delete a connect harness by OCID. Asynchronous — returns a work request OCID you can poll with Get Work Request to confirm removal."
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
	{Name: "connect_harness_ocid", Type: core.ConnectionTypeString, Label: "Connect Harness OCID", Placeholder: "ocid1.connectharness.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Connect Harness OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := str.AdminClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := str.RequiredString("connect_harness_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	resp, err := client.DeleteConnectHarness(str.Context(), streaming.DeleteConnectHarnessRequest{ConnectHarnessId: &id})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}
	return str.Result(fmt.Sprintf("Deleting connect harness %q — poll the work request to confirm", id), map[string]interface{}{
		"id":              id,
		"work_request_id": str.Str(resp.OpcWorkRequestId),
	}), nil
}

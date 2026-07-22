// Package oracle_streaming_connect_harness_get reads one connect harness by OCID — its lifecycle
// state, name and compartment. A connect harness is the endpoint Kafka Connect uses to stream
// data in and out of OCI Streaming.
package oracle_streaming_connect_harness_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Get Connect Harness"
	Description  = "Read a connect harness by OCID — its lifecycle state, name and compartment."
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
	{Name: "connect_harness", Type: core.ConnectionTypeObject, Label: "Connect Harness"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Connect Harness OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
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
	resp, err := client.GetConnectHarness(str.Context(), streaming.GetConnectHarnessRequest{ConnectHarnessId: &id})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}
	harness := str.SummariseConnectHarness(&resp.ConnectHarness)
	return str.Result(fmt.Sprintf("Connect harness %q is %s", harness["name"], harness["lifecycle_state"]), map[string]interface{}{
		"connect_harness": harness,
		"id":              harness["id"],
		"lifecycle_state": harness["lifecycle_state"],
	}), nil
}

// Package oracle_streaming_stream_pool_get reads one stream pool by OCID — its lifecycle state,
// compartment, whether it is private and its endpoint FQDN. A stream pool is the capacity and
// endpoint container that streams live inside.
package oracle_streaming_stream_pool_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Get Stream Pool"
	Description  = "Read a stream pool by OCID — its lifecycle state, compartment, privacy and endpoint FQDN."
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
	{Name: "stream_pool_ocid", Type: core.ConnectionTypeString, Label: "Stream Pool OCID", Placeholder: "ocid1.streampool.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "stream_pool", Type: core.ConnectionTypeObject, Label: "Stream Pool"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Stream Pool OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := str.AdminClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := str.RequiredString("stream_pool_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetStreamPool(str.Context(), streaming.GetStreamPoolRequest{StreamPoolId: &id})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}
	pool := str.SummariseStreamPool(&resp.StreamPool)
	return str.Result(fmt.Sprintf("Stream pool %q is %s", pool["name"], pool["lifecycle_state"]), map[string]interface{}{
		"stream_pool":     pool,
		"id":              pool["id"],
		"lifecycle_state": pool["lifecycle_state"],
	}), nil
}

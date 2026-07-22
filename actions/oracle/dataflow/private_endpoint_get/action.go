// Package oracle_dataflow_private_endpoint_get fetches a single Data Flow private endpoint by OCID,
// returning its subnet, DNS zones, host count and lifecycle state.
package oracle_dataflow_private_endpoint_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: Get Private Endpoint"
	Description  = "Fetch a single Data Flow private endpoint by its OCID — its subnet, DNS zones, host count and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "private_endpoint_ocid", Type: core.ConnectionTypeString, Label: "Private Endpoint OCID", Placeholder: "ocid1.dataflowprivateendpoint.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "private_endpoint", Type: core.ConnectionTypeObject, Label: "Private Endpoint"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Private Endpoint OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := df.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	peID, err := df.RequiredString("private_endpoint_ocid", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetPrivateEndpoint(df.Context(), dataflow.GetPrivateEndpointRequest{PrivateEndpointId: &peID})
	if err != nil {
		return df.ErrorResult(auth.OCIError(err)), nil
	}
	pe := df.SummarisePrivateEndpoint(&resp.PrivateEndpoint)
	return df.Result(fmt.Sprintf("Private endpoint %q (%s)", pe["display_name"], pe["lifecycle_state"]), map[string]interface{}{
		"private_endpoint": pe, "id": pe["id"], "lifecycle_state": pe["lifecycle_state"],
	}), nil
}

// Package oracle_generativeai_endpoint_get fetches a single Generative AI endpoint by OCID,
// returning its model, dedicated AI cluster, description and lifecycle state.
package oracle_generativeai_endpoint_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeai"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: Get Endpoint"
	Description  = "Fetch a single Generative AI endpoint by its OCID — its model, dedicated AI cluster, description and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+robot"
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
	{Name: "endpoint_ocid", Type: core.ConnectionTypeString, Label: "Endpoint OCID", Placeholder: "ocid1.generativeaiendpoint.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "endpoint", Type: core.ConnectionTypeObject, Label: "Endpoint"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Endpoint OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := gai.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	endpointID, err := gai.RequiredString("endpoint_ocid", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetEndpoint(gai.Context(), generativeai.GetEndpointRequest{EndpointId: &endpointID})
	if err != nil {
		return gai.ErrorResult(auth.OCIError(err)), nil
	}
	endpoint := gai.SummariseEndpoint(&resp.Endpoint)
	return gai.Result(fmt.Sprintf("Endpoint %q (%s)", endpoint["display_name"], endpoint["lifecycle_state"]), map[string]interface{}{
		"endpoint": endpoint, "id": endpoint["id"], "lifecycle_state": endpoint["lifecycle_state"],
	}), nil
}

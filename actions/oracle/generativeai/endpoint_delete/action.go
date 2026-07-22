// Package oracle_generativeai_endpoint_delete deletes a Generative AI endpoint by its OCID,
// tearing down the hosting for its model on the dedicated AI cluster.
package oracle_generativeai_endpoint_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeai"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: Delete Endpoint"
	Description  = "Delete a Generative AI endpoint by its OCID — it tears down the hosting for its model. Returns a work-request id to track the async teardown."
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
	{Name: "endpoint_ocid", Type: core.ConnectionTypeString, Label: "Endpoint OCID", Placeholder: "ocid1.generativeaiendpoint.oc1..aaaa… of the endpoint to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Endpoint OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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

	resp, err := client.DeleteEndpoint(gai.Context(), generativeai.DeleteEndpointRequest{EndpointId: &endpointID})
	if err != nil {
		return gai.ErrorResult(auth.OCIError(err)), nil
	}
	return gai.Result(fmt.Sprintf("Deleting endpoint %s", endpointID), map[string]interface{}{
		"id":              endpointID,
		"work_request_id": gai.Str(resp.OpcWorkRequestId),
	}), nil
}

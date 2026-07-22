// Package oracle_apigateway_gateway_change_compartment moves an API Gateway gateway from one
// compartment to another. The gateway keeps its OCID; only its compartment placement (for access
// control and billing) changes.
package oracle_apigateway_gateway_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	agw "flomation.app/automate/executor/actions/oracle/apigateway"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI API Gateway: Change Gateway Compartment"
	Description  = "Move an API Gateway gateway into a different compartment — the gateway keeps its OCID, only its compartment placement changes."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+route"
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
	{Name: "gateway_ocid", Type: core.ConnectionTypeString, Label: "Gateway OCID", Placeholder: "ocid1.apigateway.oc1..aaaa… (the gateway to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where to move the gateway)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Gateway OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := agw.GatewayClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	gatewayID, err := agw.RequiredString("gateway_ocid", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}
	destination, err := agw.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}

	_, err = client.ChangeGatewayCompartment(agw.Context(), apigateway.ChangeGatewayCompartmentRequest{
		GatewayId: &gatewayID,
		ChangeGatewayCompartmentDetails: apigateway.ChangeGatewayCompartmentDetails{
			CompartmentId: &destination,
		},
	})
	if err != nil {
		return agw.ErrorResult(auth.OCIError(err)), nil
	}

	return agw.Result(fmt.Sprintf("Moved gateway %s into compartment %s", gatewayID, destination), map[string]interface{}{
		"id":                         gatewayID,
		"destination_compartment_id": destination,
	}), nil
}

// Package oracle_apigateway_gateway_create creates an API Gateway — the virtual network appliance
// that fronts your deployed APIs on a subnet. Asynchronous: the gateway comes back CREATING with a
// work-request id; poll Get Gateway until it is ACTIVE before publishing deployments onto it.
package oracle_apigateway_gateway_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	agw "flomation.app/automate/executor/actions/oracle/apigateway"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI API Gateway: Create Gateway"
	Description  = "Create an API Gateway on a subnet. Choose a PUBLIC (internet-facing) or PRIVATE endpoint type. Returns the gateway in a CREATING state plus a work-request id — poll Get Gateway until ACTIVE."
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
	{Name: "endpoint_type", Type: core.ConnectionTypeString, Label: "Endpoint Type", Placeholder: "Public (internet-facing) or private", Required: true, Options: []core.ConnectionOption{
		{Name: "Public (internet-facing)", Value: "PUBLIC"},
		{Name: "Private (subnet-only)", Value: "PRIVATE"},
	}},
	{Name: "subnet_ocid", Type: core.ConnectionTypeString, Label: "Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… — the subnet the gateway sits on", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the gateway (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "gateway", Type: core.ConnectionTypeObject, Label: "Gateway"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Gateway OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := agw.GatewayClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}
	endpointType, err := agw.RequiredString("endpoint_type", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}
	if _, ok := apigateway.GetMappingGatewayEndpointTypeEnum(endpointType); !ok {
		return agw.ErrorResult("endpoint type must be PUBLIC or PRIVATE"), nil
	}
	subnet, err := agw.RequiredString("subnet_ocid", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}

	details := apigateway.CreateGatewayDetails{
		CompartmentId: &compartment,
		EndpointType:  apigateway.GatewayEndpointTypeEnum(endpointType),
		SubnetId:      &subnet,
	}
	if name := agw.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}

	resp, err := client.CreateGateway(agw.Context(), apigateway.CreateGatewayRequest{CreateGatewayDetails: details})
	if err != nil {
		return agw.ErrorResult(auth.OCIError(err)), nil
	}
	gateway := agw.SummariseGateway(&resp.Gateway)
	return agw.Result(fmt.Sprintf("Creating gateway %q (%s) — poll Get Gateway until ACTIVE", gateway["display_name"], gateway["lifecycle_state"]), map[string]interface{}{
		"gateway":         gateway,
		"id":              gateway["id"],
		"lifecycle_state": gateway["lifecycle_state"],
		"work_request_id": agw.Str(resp.OpcWorkRequestId),
	}), nil
}
